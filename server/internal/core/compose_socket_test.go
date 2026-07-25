package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot resolves the repository root from this test file's location so the
// checks work regardless of the test's working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// server/internal/core/compose_socket_test.go -> up 3 to repo root.
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		// CI runs `go test` with only the server/ directory mounted, so the
		// repo-root compose files are not reachable there. Skip in that case —
		// the .gitlab-ci.yml validate:compose job enforces these same
		// invariants against the full checkout. Local `go test ./...` (full
		// repo) still exercises this guard.
		if os.IsNotExist(err) {
			t.Skipf("skip: %s not reachable from test working tree (%v)", rel, err)
		}
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// TestDockerSocketMountIsRestartResilient guards the fix for the incident where
// a docker-ce package upgrade / daemon restart replaced /var/run/docker.sock
// with a new inode, stranding the long-running controller (and agent) on a
// dead, file-bind-mounted socket. The UI then falsely reported "docker not
// started / all containers down". The durable fix mounts the /run DIRECTORY and
// points DOCKER_HOST at it so the socket path resolves live on every call.
//
// If a future edit reverts to the bare socket-file mount, this test fails.
func TestDockerSocketMountIsRestartResilient(t *testing.T) {
	for _, f := range []string{"docker-compose.yml", "docker-compose.agent.yml"} {
		content := readRepoFile(t, f)
		if strings.Contains(content, "/var/run/docker.sock:/var/run/docker.sock") {
			t.Errorf("%s: found fragile socket-file bind mount "+
				"(/var/run/docker.sock:/var/run/docker.sock); mount the /run "+
				"directory instead so the socket survives a docker daemon "+
				"restart/upgrade", f)
		}
		if !strings.Contains(content, "/var/run:/host-run") {
			t.Errorf("%s: expected the /run directory mount (/var/run:/host-run)", f)
		}
		if !strings.Contains(content, `DOCKER_HOST: "unix:///host-run/docker.sock"`) {
			t.Errorf("%s: expected DOCKER_HOST pointed at the directory-mounted socket", f)
		}
	}
}

// TestServerHealthcheckVerifiesDocker guards the guardrail: the controller
// healthcheck must confirm daemon connectivity, not just the HTTP port, so a
// lost docker.sock connection surfaces as unhealthy instead of silently
// reporting healthy while the UI shows everything down.
func TestServerHealthcheckVerifiesDocker(t *testing.T) {
	dockerfile := readRepoFile(t, filepath.Join("server", "Dockerfile"))
	hc := dockerfile
	if !strings.Contains(hc, "HEALTHCHECK") {
		t.Fatal("server/Dockerfile has no HEALTHCHECK")
	}
	if !strings.Contains(hc, "docker version") {
		t.Error("server/Dockerfile HEALTHCHECK must verify docker daemon " +
			"connectivity (e.g. `docker version`) so a dead socket reports unhealthy")
	}
}
