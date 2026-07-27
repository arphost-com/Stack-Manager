package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectComposeEnvLetsDotEnvOverrideRuntimeValues(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("STATE_DIR=.stack-manager\nDB_PASSWORD=project-secret\nSTACK_UID=1234\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STATE_DIR", "/state")
	t.Setenv("DB_PASSWORD", "controller-secret")
	t.Setenv("DOCKER_HOST", "unix:///host-run/docker.sock")
	t.Setenv("COMPOSE_PROGRESS", "tty")

	env := projectComposeEnv(&Project{Name: "stack-manager", Dir: dir})
	values := envValues(env)

	if _, ok := values["STATE_DIR"]; ok {
		t.Fatalf("STATE_DIR from controller leaked into project compose environment: %q", values["STATE_DIR"])
	}
	if _, ok := values["DB_PASSWORD"]; ok {
		t.Fatalf("DB_PASSWORD from controller leaked into project compose environment")
	}
	if got := values["DOCKER_HOST"]; got != "unix:///host-run/docker.sock" {
		t.Fatalf("DOCKER_HOST = %q, want host socket", got)
	}
	if got := values["COMPOSE_PROGRESS"]; got != "plain" {
		t.Fatalf("COMPOSE_PROGRESS = %q, want plain", got)
	}
	if got, ok := values["STACK_UID"]; ok {
		t.Fatalf("STACK_UID should come from project .env, got injected value %q", got)
	}
}

func envValues(entries []string) map[string]string {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}
