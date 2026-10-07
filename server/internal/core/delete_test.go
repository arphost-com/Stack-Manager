package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testImageID = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRemoveProjectDirectoryUsesScopedPrivilegedHelperInContainer(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proxyforge")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	engine := &Engine{
		hostname: func() (string, error) { return "0123456789ab", nil },
		dockerExec: func(args ...string) (*ExecResult, error) {
			calls = append(calls, append([]string(nil), args...))
			if args[0] == "inspect" {
				return &ExecResult{Stdout: testImageID + "\n"}, nil
			}
			if err := os.RemoveAll(project); err != nil {
				t.Fatal(err)
			}
			return &ExecResult{}, nil
		},
	}

	if err := engine.removeProjectDirectory(project, root, "proxyforge"); err != nil {
		t.Fatalf("removeProjectDirectory() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("docker calls = %d, want 2", len(calls))
	}
	wantRun := []string{
		"run", "--rm", "--pull=never", "--user", "0:0",
		"--network", "none", "--read-only", "--pids-limit", "64",
		"--cap-drop", "ALL", "--cap-add", "DAC_OVERRIDE", "--cap-add", "FOWNER",
		"--security-opt", "no-new-privileges",
		"--mount", "type=bind,source=" + root + ",target=/managed-root",
		"--entrypoint", "/bin/rm", testImageID,
		"-rf", "--", "/managed-root/proxyforge",
	}
	if !reflect.DeepEqual(calls[1], wantRun) {
		t.Fatalf("docker run args = %#v, want %#v", calls[1], wantRun)
	}
}

func TestRemoveProjectDirectoryDoesNotFallBackAfterHelperFailure(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proxyforge")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}

	engine := &Engine{
		hostname: func() (string, error) { return "0123456789ab", nil },
		dockerExec: func(args ...string) (*ExecResult, error) {
			if args[0] == "inspect" {
				return &ExecResult{Stdout: testImageID}, nil
			}
			return &ExecResult{Stderr: "permission denied"}, errors.New("exit status 1")
		},
	}

	err := engine.removeProjectDirectory(project, root, "proxyforge")
	if err == nil || !strings.Contains(err.Error(), "privileged deletion helper failed: permission denied") {
		t.Fatalf("removeProjectDirectory() error = %v", err)
	}
	if _, statErr := os.Stat(project); statErr != nil {
		t.Fatalf("project directory was unexpectedly removed: %v", statErr)
	}
}

func TestDeletionHelperErrorHandlesMissingCommandResult(t *testing.T) {
	err := deletionHelperError(errors.New("docker unavailable"), nil)
	if got := err.Error(); got != "privileged deletion helper failed: docker unavailable" {
		t.Fatalf("deletionHelperError() = %q", got)
	}
}

func TestRemoveProjectDirectoryUsesNativeRemovalOutsideContainer(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proxyforge")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}

	engine := &Engine{
		hostname: func() (string, error) { return "docker02", nil },
		dockerExec: func(args ...string) (*ExecResult, error) {
			t.Fatal("docker must not be called outside a container")
			return nil, nil
		},
	}
	if err := engine.removeProjectDirectory(project, root, "proxyforge"); err != nil {
		t.Fatalf("removeProjectDirectory() error = %v", err)
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatalf("project directory still exists: %v", err)
	}
}

func TestDeleteHelperValidation(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"0123456789ab", true},
		{"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
		{"docker02", false},
		{"0123456789ag", false},
		{"0123456789a", false},
	} {
		if got := looksLikeContainerID(tc.value); got != tc.valid {
			t.Errorf("looksLikeContainerID(%q) = %v, want %v", tc.value, got, tc.valid)
		}
	}
	if !validDockerImageID(testImageID) {
		t.Fatal("valid image ID rejected")
	}
	if validDockerImageID("alpine:latest") {
		t.Fatal("tagged image reference accepted as immutable image ID")
	}
}
