package core

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type updateServiceState struct {
	name          string
	state         string
	health        string
	exitCode      int
	restarts      int
	restartPolicy string
}

// updateServiceStates reads only the Docker fields needed for post-update
// verification. In particular, it never reads container environment values.
func (e *Engine) updateServiceStates(project *Project) (map[string]updateServiceState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	filter := "label=com.docker.compose.project=" + e.getProjectName(project.Name)
	idsOut, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", filter, "--format", "{{.ID}}").Output()
	if err != nil {
		return nil, fmt.Errorf("unable to list project containers: %w", err)
	}
	ids := strings.Fields(string(idsOut))
	if len(ids) == 0 {
		return nil, fmt.Errorf("no project containers exist after Compose up")
	}
	args := append([]string{"inspect", "--format", `{{.Name}}|{{.State.Status}}|{{.State.ExitCode}}|{{.RestartCount}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}|{{.HostConfig.RestartPolicy.Name}}`}, ids...)
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("unable to inspect project container state: %w", err)
	}
	states := make(map[string]updateServiceState, len(ids))
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 6 {
			return nil, fmt.Errorf("unexpected Docker container state response")
		}
		exitCode, exitErr := strconv.Atoi(parts[2])
		restarts, restartErr := strconv.Atoi(parts[3])
		if exitErr != nil || restartErr != nil {
			return nil, fmt.Errorf("invalid Docker container state response")
		}
		name := strings.TrimPrefix(parts[0], "/")
		states[name] = updateServiceState{name, parts[1], parts[4], exitCode, restarts, parts[5]}
	}
	if len(states) != len(ids) {
		return nil, fmt.Errorf("incomplete Docker container state response")
	}
	return states, nil
}

func unstableUpdateServices(first, last map[string]updateServiceState) []string {
	var failures []string
	for name, current := range last {
		initial, existed := first[name]
		if !existed {
			failures = append(failures, name+": container changed during verification")
			continue
		}
		if current.restarts > initial.restarts {
			failures = append(failures, fmt.Sprintf("%s: restarted %d time(s) after Compose up", name, current.restarts-initial.restarts))
			continue
		}
		if current.state == "exited" && current.exitCode == 0 && current.restartPolicy == "no" {
			continue // Completed one-shot init service.
		}
		if current.state != "running" {
			failures = append(failures, name+": "+current.state)
		} else if current.health == "unhealthy" || current.health == "starting" {
			failures = append(failures, name+": "+current.health)
		}
	}
	for name := range first {
		if _, exists := last[name]; !exists {
			failures = append(failures, name+": disappeared during verification")
		}
	}
	sort.Strings(failures)
	return failures
}

func (e *Engine) verifyUpdateServices(project *Project, wait func(time.Duration)) *OpResult {
	start := time.Now()
	result := &OpResult{Project: project.Name, Action: "verify update", ExitCode: 1}
	first, err := e.updateServiceStates(project)
	if err == nil {
		// A container already restarting, dead, or exited unsuccessfully is
		// conclusively unstable; do not make the operator wait another 30 seconds.
		for _, service := range first {
			if service.state == "restarting" || service.state == "dead" || service.state == "paused" || (service.state == "exited" && service.exitCode != 0) {
				result.Output = "Post-update container verification failed:\n" + strings.Join(unstableUpdateServices(first, first), "\n") + "\n"
				result.Duration = time.Since(start).Round(time.Millisecond).String()
				return result
			}
		}
		// A short-lived "running" state is not sufficient for restart loops. The
		// second sample catches a service that Docker repeatedly restarts.
		wait(30 * time.Second)
		var last map[string]updateServiceState
		last, err = e.updateServiceStates(project)
		if err == nil {
			failures := unstableUpdateServices(first, last)
			if len(failures) > 0 {
				result.Output = "Post-update container verification failed:\n" + strings.Join(failures, "\n") + "\n"
			} else {
				result.Success = true
				result.ExitCode = 0
				result.Output = "Post-update containers stayed running or completed as one-shot services; application readiness still requires a service smoke test.\n"
			}
		}
	}
	if err != nil {
		result.Output = "Post-update container verification could not complete: " + err.Error() + "\n"
	}
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	return result
}
