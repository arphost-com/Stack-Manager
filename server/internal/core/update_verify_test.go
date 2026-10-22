package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnstableUpdateServicesDetectsRestartLoopAndKeepsCompletedInit(t *testing.T) {
	first := map[string]updateServiceState{
		"app-flowise-1": {name: "app-flowise-1", state: "running", restarts: 12, restartPolicy: "unless-stopped"},
		"app-init-1":    {name: "app-init-1", state: "exited", exitCode: 0, restartPolicy: "no"},
	}
	last := map[string]updateServiceState{
		"app-flowise-1": {name: "app-flowise-1", state: "running", restarts: 14, restartPolicy: "unless-stopped"},
		"app-init-1":    {name: "app-init-1", state: "exited", exitCode: 0, restartPolicy: "no"},
	}
	failures := unstableUpdateServices(first, last)
	if len(failures) != 1 || !strings.Contains(failures[0], "restarted 2 time(s)") {
		t.Fatalf("restart loop should fail verification without blaming completed init: %v", failures)
	}
	last["app-flowise-1"] = updateServiceState{name: "app-flowise-1", state: "running", restarts: 12, restartPolicy: "unless-stopped"}
	if failures = unstableUpdateServices(first, last); len(failures) != 0 {
		t.Fatalf("stable app and completed init should pass: %v", failures)
	}
}

func TestVerifyUpdateDetectsRunningContainerWhoseRestartCountRises(t *testing.T) {
	dir := t.TempDir()
	countFile := filepath.Join(dir, "inspect-count")
	t.Setenv("STATE_COUNTER", countFile)
	script := `#!/bin/sh
case "$1" in
  ps)
    case "$*" in
      *'{{.ID}}'*) printf 'abc123\n' ;;
      *) printf 'app\n' ;;
    esac ;;
  inspect)
    n=0
    if [ -f "$STATE_COUNTER" ]; then n=$(cat "$STATE_COUNTER"); fi
    n=$((n + 1))
    printf '%s' "$n" > "$STATE_COUNTER"
    printf '/app-flowise-1|running|0|%s|none|unless-stopped\n' "$n" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	engine := NewEngine(dir, "")
	result := engine.verifyUpdateServices(&Project{Name: "app", Dir: dir}, func(time.Duration) {})
	if result.Success || !strings.Contains(result.Output, "restarted 1 time(s)") {
		t.Fatalf("a briefly running restart loop must not pass: %+v", result)
	}
}
