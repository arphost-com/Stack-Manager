package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManualUpdateJobReportsPostUpdateFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services:\n  flowise:\n    image: example.invalid/flowise:test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "docker")
	// Fake Docker accepts Compose but reports its app service as restarting.
	// The old update path marked this as successful without reading Docker state.
	script := `#!/bin/sh
case "$1" in
  ps)
    case "$*" in
      *com.docker.compose.service*) printf 'flowise|app-flowise-1\n' ;;
      *'{{.ID}}'*) printf 'abc123\n' ;;
      *) printf 'app\n' ;;
    esac ;;
  inspect) printf '/app-flowise-1|restarting|1|3|none|unless-stopped\n' ;;
  compose)
    case "$*" in
      *"config --services"*) printf 'flowise\n' ;;
    esac ;;
esac
exit 0
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	engine := NewEngine(dir, filepath.Join(dir, "hooks"))
	project := &Project{Name: "app", Dir: dir}
	job := &ActionJob{Project: project.Name, Action: "update", Status: "running"}
	runUpdateJob(engine, project, job, 10)
	snapshot := JobSnapshot(job)
	if snapshot.Success || snapshot.ExitCode == 0 || !strings.Contains(snapshot.Output, "app-flowise-1: restarting") {
		t.Fatalf("manual update must report the unstable service: %+v", snapshot)
	}
	results := engine.Update(project, 10)
	if len(results) != 2 || results[1].Success || !strings.Contains(results[1].Output, "app-flowise-1: restarting") {
		t.Fatalf("direct update must report verification failure: %+v", results)
	}
}
