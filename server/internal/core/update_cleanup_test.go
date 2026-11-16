package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func cleanupFixture(t *testing.T, failure string) (*Engine, *Project, string) {
	t.Helper()
	dir := t.TempDir()
	trace := filepath.Join(dir, "trace")
	t.Setenv("CLEANUP_TRACE", trace)
	t.Setenv("CLEANUP_FAILURE", failure)
	t.Setenv("UPDATE_CLEANUP_ENABLED", "1")
	t.Setenv("UPDATE_CLEANUP_MIN_AGE_HOURS", "168")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$CLEANUP_TRACE"
case "$1" in
 ps) case "$*" in
 *com.docker.compose.service*) printf 'app|app-1\n';;
 *'{{.ID}}'*) printf 'abc123\n';;
 *) printf 'app\n';; esac;;
 inspect) if [ "$CLEANUP_FAILURE" = health ]; then
 printf '/app-1|restarting|1|3|none|unless-stopped\n'
 else printf '/app-1|running|0|0|healthy|unless-stopped\n'; fi;;
 compose) case "$*" in
 *'config --services'*) printf 'app\n';;
 *pull*) [ "$CLEANUP_FAILURE" != pull ] || exit 1;;
 *'up -d'*) [ "$CLEANUP_FAILURE" != up ] || exit 1;; esac;;
 image|builder) [ "$CLEANUP_FAILURE" != "$1" ] || exit 1
 printf 'Total reclaimed space: 123MB\n';;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services:\n  app:\n    image: example.invalid/app:test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	e := NewEngine(dir, filepath.Join(dir, "hooks"))
	e.verifyWait = func(time.Duration) {}
	return e, &Project{Name: "app", Dir: dir, ComposeFile: filepath.Join(dir, "compose.yml")}, trace
}

func TestUpdateCleanupWorkflows(t *testing.T) {
	for _, jobMode := range []bool{false, true} {
		for _, failure := range []string{"", "pull", "up", "health", "image", "builder", "disabled", "invalid", "controller", "hook", "hook-failed", "hook-health"} {
			t.Run(fmt.Sprintf("job=%v/%s", jobMode, failure), func(t *testing.T) {
				e, p, trace := cleanupFixture(t, failure)
				switch failure {
				case "disabled":
					t.Setenv("UPDATE_CLEANUP_ENABLED", "0")
				case "invalid":
					t.Setenv("UPDATE_CLEANUP_MIN_AGE_HOURS", "0")
				case "controller":
					p.Name = "stack-manager"
				}
				if strings.HasPrefix(failure, "hook") {
					if err := os.MkdirAll(e.HooksDir, 0700); err != nil {
						t.Fatal(err)
					}
					hook := "#!/bin/sh\nprintf 'hook\\n' >> \"$CLEANUP_TRACE\"\n"
					if failure == "hook-failed" {
						hook += "exit 1\n"
					}
					if failure == "hook-health" {
						t.Setenv("CLEANUP_FAILURE", "health")
					}
					if err := os.WriteFile(e.hookPath("post", "update", p.Name), []byte(hook), 0700); err != nil {
						t.Fatal(err)
					}
				}
				var output string
				success := true
				if jobMode {
					// Controller rejection normally lives in JobManager.run, before dispatch.
					if failure == "controller" {
						return
					}
					job := &ActionJob{Project: p.Name, Action: "update", Status: "running"}
					runUpdateJob(e, p, job, 10)
					snap := JobSnapshot(job)
					output = snap.Output
					success = snap.Success
				} else {
					for _, r := range e.Update(p, 10) {
						output += r.Output
						success = success && r.Success
					}
				}
				data, _ := os.ReadFile(trace)
				calls := string(data)
				clean := failure == "" || failure == "image" || failure == "builder" || failure == "hook"
				if strings.Contains(calls, "image prune") != clean {
					t.Fatalf("cleanup gate failed: %s\n%s", calls, output)
				}
				if clean {
					if !strings.Contains(calls, "image prune --force --filter until=168h") {
						t.Fatal(calls)
					}
					if failure != "image" && !strings.Contains(calls, "builder prune --all --force --filter until=168h --keep-storage 1GB") {
						t.Fatal(calls)
					}
					if strings.Contains(calls, "--volumes") || strings.Contains(calls, "system prune") || strings.Contains(calls, "volume prune") {
						t.Fatal(calls)
					}
					if !strings.Contains(output, "post-update cleanup") {
						t.Fatal(output)
					}
					if failure != "image" && !strings.Contains(output, "Total reclaimed space: 123MB") {
						t.Fatal(output)
					}
					if strings.Index(calls, "inspect") > strings.Index(calls, "image prune") {
						t.Fatal("cleanup preceded verification")
					}
				}
				if strings.HasPrefix(failure, "hook") && (strings.HasPrefix(calls, "compose ") || strings.Contains(calls, "\ncompose ")) {
					t.Fatal("hook override performed Compose action: " + calls)
				}
				wantSuccess := failure != "pull" && failure != "up" && failure != "health" && failure != "controller" && failure != "hook-failed"
				if success != wantSuccess {
					t.Fatalf("success=%v want=%v: %s", success, wantSuccess, output)
				}
				if (failure == "image" || failure == "builder" || failure == "invalid" || failure == "hook-health") && !strings.Contains(output, "WARNING") {
					t.Fatal(output)
				}
			})
		}
	}
}

func TestCleanupConfigAndCommandFailures(t *testing.T) {
	e, _, _ := cleanupFixture(t, "")
	for _, enabled := range []string{"0", "false", "FALSE"} {
		t.Setenv("UPDATE_CLEANUP_ENABLED", enabled)
		if got := e.cleanupAfterUpdate(); !strings.Contains(got, "disabled") {
			t.Fatal(got)
		}
	}
	t.Setenv("UPDATE_CLEANUP_ENABLED", "yes")
	if got := e.cleanupAfterUpdate(); !strings.Contains(got, "WARNING") {
		t.Fatal(got)
	}
	t.Setenv("UPDATE_CLEANUP_ENABLED", "1")
	for _, age := range []string{"-1", "23", "87601", "1h", ";anything"} {
		t.Setenv("UPDATE_CLEANUP_MIN_AGE_HOURS", age)
		if got := e.cleanupAfterUpdate(); !strings.Contains(got, "WARNING") {
			t.Fatal(got)
		}
	}
	t.Setenv("UPDATE_CLEANUP_MIN_AGE_HOURS", "24")
	var calls [][]string
	e.dockerExecWithTimeout = func(timeout time.Duration, args ...string) (*ExecResult, error) {
		if timeout != 2*time.Minute {
			t.Fatalf("unbounded cleanup: %s", timeout)
		}
		calls = append(calls, args)
		return &ExecResult{ExitCode: 124}, fmt.Errorf("timed out")
	}
	if got := e.cleanupAfterUpdate(); !strings.Contains(got, "WARNING") {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(calls, [][]string{{"image", "prune", "--force", "--filter", "until=24h"}}) {
		t.Fatal(calls)
	}
}
