package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifiedUpdateReclaimsDockerSpace(t *testing.T) {
	dir := t.TempDir()
	trace := filepath.Join(dir, "trace")
	t.Setenv("CLEANUP_TRACE", trace)
	t.Setenv("UPDATE_CLEANUP_ENABLED", "1")
	t.Setenv("UPDATE_CLEANUP_MIN_AGE_HOURS", "168")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$CLEANUP_TRACE"
case "$1" in
 ps) case "$*" in *com.docker.compose.service*) printf 'app|app-1\n';; *'{{.ID}}'*) printf 'abc123\n';; *) printf 'app\n';; esac;;
 inspect) printf '/app-1|running|0|0|healthy|unless-stopped\n';;
 compose) case "$*" in *'config --services'*) printf 'app\n';; esac;;
 image|builder) printf 'Total reclaimed space: 123MB\n';;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services:\n  app:\n    image: example.invalid/app:test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	e := NewEngine(dir, "")
	results := e.Update(&Project{Name: "app", Dir: dir, ComposeFile: filepath.Join(dir, "compose.yml")}, 10)
	var output string
	for _, r := range results {
		if !r.Success {
			t.Fatal(r)
		}
		output += r.Output
	}
	data, _ := os.ReadFile(trace)
	if !strings.Contains(string(data), "builder prune --all --force --filter until=168h --keep-storage 1GB") || !strings.Contains(output, "Total reclaimed space: 123MB") {
		t.Fatalf("verified update did not reclaim/report unused Docker space: %s\n%s", data, output)
	}
}
