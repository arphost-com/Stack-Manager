package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ActionJob struct {
	mu sync.RWMutex `json:"-"`

	ID        string    `json:"id"`
	Project   string    `json:"project"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	Success   bool      `json:"success"`
	ExitCode  int       `json:"exit_code"`
	Output    string    `json:"output"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*ActionJob
	dir  string
}

func NewJobManager(dir string) (*JobManager, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	return &JobManager{jobs: make(map[string]*ActionJob), dir: dir}, nil
}

func (m *JobManager) Start(engine *Engine, project *Project, action string, timeoutSecs int) (*ActionJob, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if !validJobAction(action) {
		return nil, fmt.Errorf("invalid action: %s", action)
	}
	id, err := randomJobID()
	if err != nil {
		return nil, err
	}
	job := &ActionJob{
		ID:        id,
		Project:   project.Name,
		Action:    action,
		Status:    "running",
		StartedAt: time.Now().UTC(),
	}

	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()

	go m.run(engine, project, job, timeoutSecs)
	return jobSnapshot(job), nil
}

func (m *JobManager) Get(id string) (*ActionJob, bool) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		job, err := m.load(id)
		if err != nil {
			return nil, false
		}
		return job, true
	}
	return jobSnapshot(job), true
}

func (m *JobManager) List() []ActionJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]ActionJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, *jobSnapshot(job))
	}
	entries, err := os.ReadDir(m.dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".json")
			if _, ok := m.jobs[id]; ok {
				continue
			}
			if job, err := m.load(id); err == nil {
				jobs = append(jobs, *job)
			}
		}
	}
	return jobs
}

func (m *JobManager) run(engine *Engine, project *Project, job *ActionJob, timeoutSecs int) {
	defer func() {
		updateJob(job, func(j *ActionJob) {
			j.EndedAt = time.Now().UTC()
			j.Duration = j.EndedAt.Sub(j.StartedAt).Round(time.Millisecond).String()
			if j.Status == "running" {
				j.Status = "completed"
			}
		})
		_ = m.save(job)
	}()

	switch job.Action {
	case "pull":
		success, exitCode := runComposeJob(engine, project, job, timeoutSecs, "pull")
		updateJob(job, func(j *ActionJob) {
			j.Success = success
			j.ExitCode = exitCode
		})
	case "up":
		success, exitCode := runComposeJob(engine, project, job, 0, "up", "-d")
		updateJob(job, func(j *ActionJob) {
			j.Success = success
			j.ExitCode = exitCode
		})
	case "down":
		success, exitCode := runComposeJob(engine, project, job, 0, "down")
		updateJob(job, func(j *ActionJob) {
			j.Success = success
			j.ExitCode = exitCode
		})
	case "restart":
		success, exitCode := runComposeJob(engine, project, job, 0, "restart")
		updateJob(job, func(j *ActionJob) {
			j.Success = success
			j.ExitCode = exitCode
		})
	case "status":
		success, exitCode := runComposeJob(engine, project, job, 0, "ps")
		updateJob(job, func(j *ActionJob) {
			j.Success = success
			j.ExitCode = exitCode
		})
	case "update":
		runUpdateJob(engine, project, job, timeoutSecs)
	}
	if !jobSnapshot(job).Success {
		updateJob(job, func(j *ActionJob) {
			j.Status = "failed"
		})
	}
}

func (m *JobManager) load(id string) (*ActionJob, error) {
	data, err := os.ReadFile(filepath.Join(m.dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var job ActionJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (m *JobManager) save(job *ActionJob) error {
	data, err := json.MarshalIndent(jobSnapshot(job), "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(m.dir, job.ID+".json.tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(m.dir, job.ID+".json"))
}

func runUpdateJob(engine *Engine, project *Project, job *ActionJob, timeoutSecs int) {
	if engine.HasHook("post", "update", project.Name) {
		appendJobOutput(job, "=== update hook ===\n")
		path := engine.hookPath("post", "update", project.Name)
		success, exitCode := runCommandJob(job, project.Dir, 0, path, project.Name, project.Dir)
		updateJob(job, func(j *ActionJob) {
			j.Success = success
			j.ExitCode = exitCode
		})
		return
	}

	appendJobOutput(job, "=== docker compose pull ===\n")
	pullOK, pullExit := runComposeJob(engine, project, job, timeoutSecs, "pull")
	if !pullOK {
		updateJob(job, func(j *ActionJob) {
			j.Success = false
			j.ExitCode = pullExit
		})
		return
	}
	appendJobOutput(job, "\n=== docker compose up -d ===\n")
	upOK, upExit := runComposeJob(engine, project, job, 0, "up", "-d")
	updateJob(job, func(j *ActionJob) {
		j.Success = upOK
		j.ExitCode = upExit
	})
}

func runComposeJob(engine *Engine, project *Project, job *ActionJob, timeoutSecs int, args ...string) (bool, int) {
	pname := engine.getProjectName(project.Name)
	composeArgs := []string{"compose", "-f", project.ComposeFile, "-p", pname}
	composeArgs = append(composeArgs, args...)
	return runCommandJob(job, project.Dir, timeoutSecs, "docker", composeArgs...)
}

func runCommandJob(job *ActionJob, dir string, timeoutSecs int, name string, args ...string) (bool, int) {
	timeout := 5 * time.Minute
	if timeoutSecs > 0 {
		timeout = time.Duration(timeoutSecs) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "COMPOSE_PROGRESS=plain")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		setJobError(job, err.Error())
		appendJobOutput(job, err.Error()+"\n")
		return false, 1
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		setJobError(job, err.Error())
		appendJobOutput(job, err.Error()+"\n")
		return false, 1
	}

	if err := cmd.Start(); err != nil {
		setJobError(job, err.Error())
		appendJobOutput(job, err.Error()+"\n")
		return false, 1
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go copyJobOutput(job, stdout, &wg)
	go copyJobOutput(job, stderr, &wg)

	err = cmd.Wait()
	wg.Wait()

	if ctx.Err() == context.DeadlineExceeded {
		setJobError(job, "command timed out")
		appendJobOutput(job, "\ncommand timed out\n")
		return false, 1
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, exitErr.ExitCode()
		}
		setJobError(job, err.Error())
		return false, 1
	}
	return true, 0
}

func copyJobOutput(job *ActionJob, reader io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			appendJobOutput(job, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func appendJobOutput(job *ActionJob, text string) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.Output += text
	const maxOutput = 1024 * 1024
	if len(job.Output) > maxOutput {
		job.Output = job.Output[len(job.Output)-maxOutput:]
	}
}

func jobSnapshot(job *ActionJob) *ActionJob {
	job.mu.RLock()
	defer job.mu.RUnlock()

	cp := &ActionJob{
		ID:        job.ID,
		Project:   job.Project,
		Action:    job.Action,
		Status:    job.Status,
		Success:   job.Success,
		ExitCode:  job.ExitCode,
		Output:    job.Output,
		StartedAt: job.StartedAt,
		EndedAt:   job.EndedAt,
		Duration:  job.Duration,
		Error:     job.Error,
	}
	return cp
}

func updateJob(job *ActionJob, fn func(*ActionJob)) {
	job.mu.Lock()
	defer job.mu.Unlock()
	fn(job)
}

func setJobError(job *ActionJob, msg string) {
	updateJob(job, func(j *ActionJob) {
		j.Error = msg
	})
}

func validJobAction(action string) bool {
	switch action {
	case "pull", "up", "down", "restart", "status", "update":
		return true
	default:
		return false
	}
}

func randomJobID() (string, error) {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func hookExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0111 != 0
}
