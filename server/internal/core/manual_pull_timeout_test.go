package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManualPullWithoutTimeoutCanFinishAfterExplicitLimit(t *testing.T) {
	ctx, cancel := commandJobContext(0)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("manual pull with timeout 0 must have no deadline")
	}
	limited, cancelLimited := commandJobContext(1)
	defer cancelLimited()
	if _, ok := limited.Deadline(); !ok {
		t.Fatal("explicit pull timeout must retain its deadline")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services:\n  app:\n    image: example.invalid/app:test\n  worker:\n    build: .\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Simulate a registry pull that takes longer than the operator's one-second
	// limit. The zero-timeout manual job must still report completion and output.
	docker := filepath.Join(dir, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nsleep 2\nprintf 'image downloaded %s\\n' \"$*\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	engine := NewEngine(dir, filepath.Join(dir, "hooks"))
	project := &Project{Name: "app", Dir: dir, ImageSources: []ImageSource{{Service: "worker", Build: true}}}
	m := NewJobManager(nil)
	job, err := m.Start(engine, project, "pull", 0)
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForTestJob(t, m, job.ID)
	if finished.Status != "completed" || !finished.Success || !strings.Contains(finished.Output, "image downloaded") || !strings.Contains(finished.Output, "--ignore-buildable") {
		t.Fatalf("unlimited manual pull should finish: status=%s success=%v output=%q", finished.Status, finished.Success, finished.Output)
	}

	limitedJob, err := m.Start(engine, project, "pull", 1)
	if err != nil {
		t.Fatal(err)
	}
	finished = waitForTestJob(t, m, limitedJob.ID)
	if finished.Status != "failed" || finished.Error != "command timed out" {
		t.Fatalf("explicit limit should stop pull: status=%s error=%q", finished.Status, finished.Error)
	}
}

func waitForTestJob(t *testing.T, m *JobManager, id string) *ActionJob {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := m.Get(id)
		if ok && job.Status != "running" {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return nil
}
