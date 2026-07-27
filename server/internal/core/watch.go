package core

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// WatchSession is one persisted "Up + Watch" run. The raw log stream lives at
// LogPath so refreshing the browser mid-stream still replays cleanly from
// disk. Only new bytes appended after the last-read offset are streamed to
// subsequent SSE subscribers.
type WatchSession struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	ExitCode  int       `json:"exit_code"`
	Running   bool      `json:"running"`
	SizeBytes int64     `json:"size_bytes"`
	LogPath   string    `json:"-"`
	MetaPath  string    `json:"-"`
}

// WatchManager owns the disk layout and the map of currently-running compose
// logs subprocesses. Every project can have at most one live session at a
// time - starting a new one gracefully closes the previous subprocess so
// idle tailers do not accumulate on the host.
type WatchManager struct {
	engine    *Engine
	root      string
	perProj   int
	mu        sync.Mutex
	running   map[string]*runningWatch // key: project + "/" + session id
	perActive map[string]string        // key: project → active session id
}

type runningWatch struct {
	session *WatchSession
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewWatchManager configures the manager rooted at STATE_DIR/logs.
func NewWatchManager(engine *Engine, stateDir string) *WatchManager {
	root := filepath.Join(stateDir, "logs")
	_ = os.MkdirAll(root, 0o750)
	return &WatchManager{
		engine:    engine,
		root:      root,
		perProj:   20,
		running:   map[string]*runningWatch{},
		perActive: map[string]string{},
	}
}

var sessionIDRe = regexp.MustCompile(`^[a-f0-9]{16}$`)

func newSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// projectDir enforces that the project name resolves to a real project so we
// never write log files for an arbitrary user-supplied path.
func (w *WatchManager) projectDir(name string) (string, *Project, error) {
	if strings.TrimSpace(name) == "" {
		return "", nil, fmt.Errorf("project name is required")
	}
	proj, err := w.engine.GetProject(name)
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(w.root, proj.Name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", nil, err
	}
	return dir, proj, nil
}

// Start kicks off `docker compose up -d` synchronously (fast enough to block
// the request), then spawns a background `docker compose logs -f` writing to
// the session's log file. Returns the session record for the client to
// subscribe against.
func (w *WatchManager) Start(name string) (*WatchSession, string, error) {
	dir, project, err := w.projectDir(name)
	if err != nil {
		return nil, "", err
	}
	id, err := newSessionID()
	if err != nil {
		return nil, "", err
	}
	session := &WatchSession{
		ID:        id,
		Project:   project.Name,
		StartedAt: time.Now().UTC(),
		Running:   true,
		LogPath:   filepath.Join(dir, id+".log"),
		MetaPath:  filepath.Join(dir, id+".json"),
	}

	// docker compose up -d first. Any pull happens here as part of normal
	// compose behaviour (missing images only, per pull_policy).
	upResult := w.engine.Up(project)
	upOutput := "=== docker compose up -d ===\n" + upResult.Output + "\n"
	if err := os.WriteFile(session.LogPath, []byte(upOutput), 0o640); err != nil {
		return nil, "", err
	}
	if !upResult.Success {
		// If up failed there is nothing to tail. Record the failure and
		// return - the client will still open the SSE stream, replay the
		// up output, and see the "ended" event immediately.
		session.EndedAt = time.Now().UTC()
		session.Running = false
		session.ExitCode = upResult.ExitCode
		if info, statErr := os.Stat(session.LogPath); statErr == nil {
			session.SizeBytes = info.Size()
		}
		w.writeMeta(session)
		w.rotate(dir)
		return session, upOutput, nil
	}

	// Kill any prior running session for this project so we do not stack
	// zombie `docker compose logs -f` processes on the host.
	w.mu.Lock()
	if prevID, ok := w.perActive[project.Name]; ok {
		if prev, ok := w.running[project.Name+"/"+prevID]; ok {
			prev.cancel()
			delete(w.running, project.Name+"/"+prevID)
		}
		delete(w.perActive, project.Name)
	}
	w.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	logArgs := []string{"compose"}
	logArgs = append(logArgs, composeFileArgs(project)...)
	logArgs = append(logArgs, "-p", w.engine.getProjectName(project.Name), "logs", "-f", "--no-color", "--timestamps", "--since=0s")
	logsCmd := exec.CommandContext(ctx, "docker", logArgs...)
	logsCmd.Dir = project.Dir
	logsCmd.Env = projectComposeEnv(project)

	f, err := os.OpenFile(session.LogPath, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		cancel()
		return nil, "", err
	}
	// docker compose logs writes to both stdout and stderr; merge them into
	// the session file so the client sees the full startup output.
	logsCmd.Stdout = f
	logsCmd.Stderr = f

	if err := logsCmd.Start(); err != nil {
		f.Close()
		cancel()
		return nil, "", err
	}

	w.mu.Lock()
	w.running[project.Name+"/"+id] = &runningWatch{session: session, cancel: cancel, done: done}
	w.perActive[project.Name] = id
	w.mu.Unlock()

	go func() {
		defer close(done)
		defer f.Close()
		err := logsCmd.Wait()
		w.mu.Lock()
		session.EndedAt = time.Now().UTC()
		session.Running = false
		session.ExitCode = 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				session.ExitCode = exitErr.ExitCode()
			} else {
				session.ExitCode = 1
			}
		}
		if info, statErr := os.Stat(session.LogPath); statErr == nil {
			session.SizeBytes = info.Size()
		}
		w.writeMeta(session)
		delete(w.running, project.Name+"/"+id)
		if w.perActive[project.Name] == id {
			delete(w.perActive, project.Name)
		}
		w.mu.Unlock()
	}()

	w.writeMeta(session)
	w.rotate(dir)
	return session, upOutput, nil
}

// Stop kills a running session's log follower. The log file is preserved so
// clients can still replay history.
func (w *WatchManager) Stop(projectName, sessionID string) error {
	if !sessionIDRe.MatchString(sessionID) {
		return fmt.Errorf("invalid session id")
	}
	w.mu.Lock()
	rw, ok := w.running[projectName+"/"+sessionID]
	w.mu.Unlock()
	if !ok {
		return nil
	}
	rw.cancel()
	<-rw.done
	return nil
}

// Sessions returns metadata for every persisted session of a project, newest
// first. Used to populate the "Reopen a past run" dropdown.
func (w *WatchManager) Sessions(projectName string) ([]WatchSession, error) {
	if _, _, err := w.projectDir(projectName); err != nil {
		return nil, err
	}
	dir := filepath.Join(w.root, projectName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []WatchSession{}, nil
		}
		return nil, err
	}
	out := []WatchSession{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !sessionIDRe.MatchString(id) {
			continue
		}
		session, err := w.LoadSession(projectName, id)
		if err != nil {
			continue
		}
		out = append(out, *session)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

// LoadSession returns the on-disk record for a specific session.
func (w *WatchManager) LoadSession(projectName, sessionID string) (*WatchSession, error) {
	if !sessionIDRe.MatchString(sessionID) {
		return nil, fmt.Errorf("invalid session id")
	}
	dir := filepath.Join(w.root, projectName)
	meta := filepath.Join(dir, sessionID+".json")
	raw, err := os.ReadFile(meta)
	if err != nil {
		return nil, err
	}
	var s WatchSession
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	s.LogPath = filepath.Join(dir, sessionID+".log")
	s.MetaPath = meta
	if info, err := os.Stat(s.LogPath); err == nil {
		s.SizeBytes = info.Size()
	}
	// A meta file marked running might be stale if the server crashed - if
	// we no longer track a live process for it, force Running=false so the
	// UI does not spin forever waiting for a subprocess that never
	// existed.
	if s.Running {
		w.mu.Lock()
		_, alive := w.running[projectName+"/"+sessionID]
		w.mu.Unlock()
		if !alive {
			s.Running = false
		}
	}
	return &s, nil
}

// Stream reads the session's log file and pipes it into out. When the
// session is still running it polls the file for new bytes and forwards
// them until the process exits, the client disconnects (ctx cancels), or
// the idle deadline hits.
func (w *WatchManager) Stream(ctx context.Context, projectName, sessionID string, out io.Writer, flusher func()) error {
	session, err := w.LoadSession(projectName, sessionID)
	if err != nil {
		return err
	}
	f, err := os.Open(session.LogPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Replay everything that is on disk right now.
	reader := bufio.NewReader(f)
	if err := copyLines(reader, out, flusher); err != nil {
		return err
	}
	if !session.Running {
		return nil
	}

	// Live tail: poll for new bytes every 500ms. Cheap enough (single
	// syscall per tick) and it avoids needing inotify in the alpine
	// container.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	idle := time.NewTimer(30 * time.Minute)
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-idle.C:
			return nil
		case <-ticker.C:
			if err := copyLines(reader, out, flusher); err != nil {
				return err
			}
			// Refresh live-ness from the manager rather than the stale meta.
			w.mu.Lock()
			_, alive := w.running[projectName+"/"+sessionID]
			w.mu.Unlock()
			if !alive {
				// Read any bytes written between the last tick and the
				// process exit so we don't cut off the tail.
				_ = copyLines(reader, out, flusher)
				return nil
			}
			// Reset the idle timer whenever the subscriber is actually
			// consuming bytes (implicit here - we sent bytes in copyLines).
			if !idle.Stop() {
				<-idle.C
			}
			idle.Reset(30 * time.Minute)
		}
	}
}

func copyLines(reader *bufio.Reader, out io.Writer, flusher func()) error {
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if _, werr := out.Write([]byte(line)); werr != nil {
				return werr
			}
			if flusher != nil {
				flusher()
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// Shutdown stops every running session on server exit so the subprocesses
// terminate cleanly. Called from main during graceful shutdown.
func (w *WatchManager) Shutdown() {
	w.mu.Lock()
	watches := make([]*runningWatch, 0, len(w.running))
	for _, rw := range w.running {
		watches = append(watches, rw)
	}
	w.mu.Unlock()
	for _, rw := range watches {
		rw.cancel()
	}
	for _, rw := range watches {
		select {
		case <-rw.done:
		case <-time.After(3 * time.Second):
		}
	}
}

func (w *WatchManager) writeMeta(s *WatchSession) {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.MetaPath, raw, 0o640)
}

// rotate deletes the oldest sessions until at most perProj remain per project.
// Simple count-based rotation - no cron needed.
func (w *WatchManager) rotate(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type sessionFile struct {
		id  string
		mod time.Time
	}
	sessions := []sessionFile{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !sessionIDRe.MatchString(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionFile{id: id, mod: info.ModTime()})
	}
	if len(sessions) <= w.perProj {
		return
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].mod.Before(sessions[j].mod) })
	toDelete := sessions[:len(sessions)-w.perProj]
	for _, s := range toDelete {
		_ = os.Remove(filepath.Join(dir, s.id+".log"))
		_ = os.Remove(filepath.Join(dir, s.id+".json"))
	}
}
