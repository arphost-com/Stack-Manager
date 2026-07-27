package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	mu                 sync.RWMutex
	jobs               map[string]*ActionJob
	store              JobStore
	completionHandlers []func(*ActionJob)
}

type JobStore interface {
	SaveJob(context.Context, *ActionJob) error
	LoadJob(context.Context, string) (*ActionJob, error)
	ListJobs(context.Context) ([]ActionJob, error)
}

func NewJobManager(store JobStore) *JobManager {
	return &JobManager{jobs: make(map[string]*ActionJob), store: store}
}

// AddCompletionHandler registers a callback invoked after every tracked job.
// Manual jobs also invoke it; handlers must safely ignore jobs they do not own.
func (m *JobManager) AddCompletionHandler(handler func(*ActionJob)) {
	m.mu.Lock()
	m.completionHandlers = append(m.completionHandlers, handler)
	m.mu.Unlock()
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
	return JobSnapshot(job), nil
}

func (m *JobManager) StartSkipped(project *Project, action, output string) (*ActionJob, error) {
	id, err := randomJobID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	job := &ActionJob{
		ID:        id,
		Project:   project.Name,
		Action:    action,
		Status:    "skipped",
		Success:   true,
		ExitCode:  0,
		Output:    output,
		StartedAt: now,
		EndedAt:   now,
		Duration:  "0s",
	}
	if m.store != nil {
		_ = m.store.SaveJob(context.Background(), job)
	}
	return JobSnapshot(job), nil
}

func (m *JobManager) Get(id string) (*ActionJob, bool) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		if m.store == nil {
			return nil, false
		}
		job, err := m.store.LoadJob(context.Background(), id)
		if err != nil {
			return nil, false
		}
		return job, true
	}
	return JobSnapshot(job), true
}

func (m *JobManager) List() []ActionJob {
	m.mu.RLock()
	jobs := make([]ActionJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, *JobSnapshot(job))
	}
	live := make(map[string]struct{}, len(m.jobs))
	for id := range m.jobs {
		live[id] = struct{}{}
	}
	m.mu.RUnlock()

	if m.store != nil {
		stored, err := m.store.ListJobs(context.Background())
		if err == nil {
			for _, job := range stored {
				if _, ok := live[job.ID]; !ok {
					jobs = append(jobs, job)
				}
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
		m.mu.RLock()
		handlers := append([]func(*ActionJob){}, m.completionHandlers...)
		m.mu.RUnlock()
		for _, handler := range handlers {
			handler(JobSnapshot(job))
		}
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
	if !JobSnapshot(job).Success {
		updateJob(job, func(j *ActionJob) {
			j.Status = "failed"
		})
	}
}

func (m *JobManager) save(job *ActionJob) error {
	if m.store == nil {
		return nil
	}
	return m.store.SaveJob(context.Background(), job)
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
	composeArgs := []string{"compose"}
	composeArgs = append(composeArgs, composeFileArgs(project)...)
	composeArgs = append(composeArgs, "-p", pname)
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

	// Callers pass fixed Docker commands or validated hook paths; arguments are not shell-expanded.
	cmd := exec.CommandContext(ctx, name, args...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "COMPOSE_PROGRESS=plain")
	if job != nil && job.Project != "" {
		cmd.Env = append(cmd.Env, stackManagerUserEnv(&Project{Name: job.Project, Dir: dir})...)
	}

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

func JobSnapshot(job *ActionJob) *ActionJob {
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
	return ValidJobAction(action)
}

func ValidJobAction(action string) bool {
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
