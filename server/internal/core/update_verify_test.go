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

// writeHealthSequenceDocker installs a fake docker binary whose inspect output
// reports a running container with the given health on successive calls. The
// last entry repeats once the sequence is exhausted.
func writeHealthSequenceDocker(t *testing.T, health ...string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("STATE_COUNTER", filepath.Join(dir, "inspect-count"))
	t.Setenv("HEALTH_SEQUENCE", strings.Join(health, " "))
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
    i=0; h=""
    for s in $HEALTH_SEQUENCE; do i=$((i + 1)); h=$s; [ "$i" -ge "$n" ] && break; done
    printf '/app-gitlab-1|running|0|0|%s|unless-stopped\n' "$h" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestVerifyUpdateWaitsForSlowHealthcheck(t *testing.T) {
	dir := writeHealthSequenceDocker(t, "starting", "starting", "starting", "healthy")
	var waited time.Duration
	engine := NewEngine(dir, "")
	result := engine.verifyUpdateServices(&Project{Name: "app", Dir: dir}, func(d time.Duration) { waited += d })
	if !result.Success {
		t.Fatalf("a service that becomes healthy within the wait budget must pass: %+v", result)
	}
	if waited != updateVerifySettle+2*updateVerifyHealthPoll {
		t.Fatalf("expected settle plus two polls, waited %s", waited)
	}
}

func TestVerifyUpdateFailsWhenHealthcheckNeverFinishesStarting(t *testing.T) {
	dir := writeHealthSequenceDocker(t, "starting")
	var waited time.Duration
	engine := NewEngine(dir, "")
	result := engine.verifyUpdateServices(&Project{Name: "app", Dir: dir}, func(d time.Duration) { waited += d })
	if result.Success || !strings.Contains(result.Output, "healthcheck still starting after") {
		t.Fatalf("a healthcheck stuck in starting must fail with a clear reason: %+v", result)
	}
	if waited < updateVerifyHealthWait {
		t.Fatalf("verification gave up after %s, before the %s budget", waited, updateVerifyHealthWait)
	}
}

func TestVerifyUpdateStillFailsUnhealthyWithoutWaiting(t *testing.T) {
	dir := writeHealthSequenceDocker(t, "starting", "unhealthy")
	var waited time.Duration
	engine := NewEngine(dir, "")
	result := engine.verifyUpdateServices(&Project{Name: "app", Dir: dir}, func(d time.Duration) { waited += d })
	if result.Success || !strings.Contains(result.Output, "app-gitlab-1: unhealthy") {
		t.Fatalf("an unhealthy service must fail: %+v", result)
	}
	if waited != updateVerifySettle {
		t.Fatalf("unhealthy should fail at the settle sample, waited %s", waited)
	}
}

func TestOnlyHealthStarting(t *testing.T) {
	cases := []struct {
		failures []string
		want     bool
	}{
		{nil, false},
		{[]string{"a: starting"}, true},
		{[]string{"a: starting", "b: starting"}, true},
		{[]string{"a: starting", "b: unhealthy"}, false},
		{[]string{"a: restarted 1 time(s) after Compose up"}, false},
	}
	for _, c := range cases {
		if got := onlyHealthStarting(c.failures); got != c.want {
			t.Errorf("onlyHealthStarting(%v) = %v, want %v", c.failures, got, c.want)
		}
	}
}
