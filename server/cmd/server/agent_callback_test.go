package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

func TestCallbackAgentRunsHostPruneWithoutProjectLookup(t *testing.T) {
	engine := core.NewEngine(t.TempDir(), "")
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "docker-args")
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.WriteFile(dockerPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOCKER_ARGS_FILE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_ARGS_FILE", argsFile)
	success, output := executeAgentCommand(context.Background(), engine, core.AgentCommandDispatch{Action: "prune-system-all", Project: "(system)"})
	if !success {
		t.Fatalf("prune failed: %s", output)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Fields(string(raw)), []string{"system", "prune", "--all", "--volumes", "-f"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("docker args = %#v, want %#v", got, want)
	}
}
