package firewall

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker installs a docker binary that counts its invocations and fails
// the way the helper does on a host without csf.
func fakeDocker(t *testing.T, stderr string, exitCode string) string {
	t.Helper()
	dir := t.TempDir()
	counter := filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		"n=0; [ -f '" + counter + "' ] && n=$(cat '" + counter + "'); echo $((n + 1)) > '" + counter + "'\n" +
		"printf '%s\\n' '" + stderr + "' >&2\n" +
		"exit " + exitCode + "\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return counter
}

func calls(t *testing.T, counter string) string {
	t.Helper()
	b, err := os.ReadFile(counter)
	if err != nil {
		return "0"
	}
	return strings.TrimSpace(string(b))
}

func TestAllowIPIsQuietAndCachedWhenCSFIsNotInstalled(t *testing.T) {
	counter := fakeDocker(t, "[stack-manager-csf] ERROR: csf is not installed at /usr/sbin/csf", "2")
	s := New()
	for i := 0; i < 3; i++ {
		if err := s.AllowIP(context.Background(), "203.0.113.7", "Stack Manager login barry"); err != nil {
			t.Fatalf("login %d: a host without csf must not report an error: %v", i+1, err)
		}
	}
	if got := calls(t, counter); got != "1" {
		t.Fatalf("helper should run once and then be skipped, ran %s times", got)
	}
}

func TestAllowIPStillReportsRealHelperFailures(t *testing.T) {
	fakeDocker(t, "[stack-manager-csf] ERROR: csf -a failed", "2")
	s := New()
	err := s.AllowIP(context.Background(), "203.0.113.7", "Stack Manager login barry")
	if err == nil || !strings.Contains(err.Error(), "csf -a failed") {
		t.Fatalf("a real csf failure must still be returned, got %v", err)
	}
}
