package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHostHelperEnvPropagatesDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///host-run/docker.sock")
	env := HostHelperEnv()
	var hasPath, hasDockerHost bool
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
		}
		if e == "DOCKER_HOST=unix:///host-run/docker.sock" {
			hasDockerHost = true
		}
	}
	if !hasPath {
		t.Error("HostHelperEnv must always include PATH")
	}
	if !hasDockerHost {
		t.Error("HostHelperEnv must propagate DOCKER_HOST so the privileged " +
			"docker-run helpers can reach the daemon via the /host-run socket")
	}

	// With DOCKER_HOST unset, it must not fabricate one.
	t.Setenv("DOCKER_HOST", "")
	os.Unsetenv("DOCKER_HOST")
	for _, e := range HostHelperEnv() {
		if strings.HasPrefix(e, "DOCKER_HOST=") {
			t.Errorf("HostHelperEnv fabricated %q when DOCKER_HOST was unset", e)
		}
	}
}

// TestHelperCallSitesUseSharedEnv guards against re-inlining the PATH-only
// scrubbed env at a `docker run` helper call site. Every such site must build
// its env via HostHelperEnv() so DOCKER_HOST is always carried; a raw literal
// silently drops it and breaks the helper ("Cannot connect to the Docker
// daemon at unix:///var/run/docker.sock") now that the socket lives at
// /host-run inside the container.
func TestHelperCallSitesUseSharedEnv(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	internalDir := filepath.Join(filepath.Dir(thisFile), "..") // server/internal
	sites := []string{
		"handlers/self_update.go",
		"handlers/gpu_setup.go",
		"handlers/os_update.go",
		"handlers/system_tz.go",
		"skills/firewall/firewall.go",
	}
	const forbidden = `[]string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}`
	for _, rel := range sites {
		b, err := os.ReadFile(filepath.Join(internalDir, rel))
		if err != nil {
			t.Errorf("read %s: %v", rel, err)
			continue
		}
		if strings.Contains(string(b), forbidden) {
			t.Errorf("%s: uses a raw PATH-only scrubbed env for a docker helper; "+
				"use core.HostHelperEnv() so DOCKER_HOST is carried", rel)
		}
	}
}
