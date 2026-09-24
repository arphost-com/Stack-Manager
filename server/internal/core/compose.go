package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// getProjectName returns the compose project name, preferring an existing label.
func (e *Engine) getProjectName(name string) string {
	// Try to detect from existing containers first. Include stopped and
	// restarting containers because their Compose project label is still the
	// authoritative name.
	if label := e.detectRunningLabel(name); label != "" {
		return label
	}
	return sanitizeProjectName(name)
}

// detectRunningLabel checks if containers exist with a compose project label.
func (e *Engine) detectRunningLabel(name string) string {
	sanitized := sanitizeProjectName(name)
	candidates := []string{sanitized, strings.ToLower(name)}

	for _, cand := range candidates {
		out, err := exec.Command("docker", "ps", "-a",
			"--filter", fmt.Sprintf("label=com.docker.compose.project=%s", cand),
			"--format", `{{.Label "com.docker.compose.project"}}`,
		).Output()
		if err == nil {
			label := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			if label != "" {
				return label
			}
		}
	}
	return ""
}

// getContainers returns every container for a project, including transitional
// and stopped containers. The second return value is the aggregate project
// state derived from Docker's live container states.
func (e *Engine) getContainers(name string) ([]Container, string) {
	pname := e.getProjectName(name)
	out, err := exec.Command("docker", "ps", "-a",
		"--filter", fmt.Sprintf("label=com.docker.compose.project=%s", pname),
		"--format", `{{json .}}`,
	).Output()
	if err != nil {
		return nil, "unknown"
	}

	var containers []Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var raw struct {
			ID    string `json:"ID"`
			Names string `json:"Names"`
			Image string `json:"Image"`
			State string `json:"State"`
			Ports string `json:"Ports"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		containers = append(containers, Container{
			ID:    raw.ID,
			Name:  raw.Names,
			Image: raw.Image,
			State: raw.State,
			Ports: raw.Ports,
		})
	}

	return containers, aggregateProjectState(containers)
}

// aggregateProjectState reports the operator-relevant state for a Compose
// project. A restart loop must win over a concurrently running sidecar, while
// exited one-shot init containers must not make an otherwise running project
// look stopped.
func aggregateProjectState(containers []Container) string {
	if len(containers) == 0 {
		return "stopped"
	}

	present := make(map[string]bool, len(containers))
	for _, container := range containers {
		state := strings.ToLower(strings.TrimSpace(container.State))
		if state != "" {
			present[state] = true
		}
	}

	for _, state := range []string{"restarting", "dead", "removing", "paused", "running", "created", "exited"} {
		if present[state] {
			return state
		}
	}
	return "unknown"
}

// ExecCompose runs a docker compose command for a project.
func (e *Engine) ExecCompose(project *Project, args ...string) *OpResult {
	return e.ExecComposeWithTimeout(project, 0, args...)
}

// ExecComposeWithTimeout runs a docker compose command with a timeout.
func (e *Engine) ExecComposeWithTimeout(project *Project, timeoutSecs int, args ...string) *OpResult {
	pname := e.getProjectName(project.Name)

	composeArgs := []string{"compose"}
	composeArgs = append(composeArgs, composeFileArgs(project)...)
	composeArgs = append(composeArgs, "-p", pname)
	composeArgs = append(composeArgs, args...)

	var ctx context.Context
	var cancel context.CancelFunc
	if timeoutSecs > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	}
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "docker", composeArgs...)
	cmd.Dir = project.Dir
	cmd.Env = projectComposeEnv(project)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &OpResult{
		Project:  project.Name,
		Action:   strings.Join(args, " "),
		Output:   stdout.String() + stderr.String(),
		Duration: duration.Round(time.Millisecond).String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Success = false
	} else {
		result.ExitCode = 0
		result.Success = true
	}

	return result
}

// CheckComposeServiceIdentity refuses an update when containers from the
// existing Compose project belong to services that are no longer present in
// the current Compose model. A service rename otherwise turns the old
// container into an orphan and `up -d` may start a replacement that collides
// on ports or mounts. The operator must reconcile that migration explicitly.
func (e *Engine) CheckComposeServiceIdentity(project *Project) *OpResult {
	start := time.Now()
	result := &OpResult{Project: project.Name, Action: "compose service identity preflight"}
	pname := e.getProjectName(project.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	composeArgs := []string{"compose"}
	composeArgs = append(composeArgs, composeFileArgs(project)...)
	composeArgs = append(composeArgs, "-p", pname, "config", "--services")
	cmd := exec.CommandContext(ctx, "docker", composeArgs...)
	cmd.Dir = project.Dir
	cmd.Env = projectComposeEnv(project)
	configuredOut, err := cmd.CombinedOutput()
	if err != nil {
		result.ExitCode = commandExitCode(err)
		result.Output = "unable to resolve current Compose services: " + err.Error() + "\n" + string(configuredOut)
		result.Duration = time.Since(start).Round(time.Millisecond).String()
		return result
	}

	containersOut, err := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", fmt.Sprintf("label=com.docker.compose.project=%s", pname),
		"--format", `{{.Label "com.docker.compose.service"}}|{{.Names}}`,
	).CombinedOutput()
	if err != nil {
		result.ExitCode = commandExitCode(err)
		result.Output = "unable to inspect existing Compose service labels: " + err.Error() + "\n" + string(containersOut)
		result.Duration = time.Since(start).Round(time.Millisecond).String()
		return result
	}

	orphans := findOrphanComposeServices(strings.Fields(string(configuredOut)), string(containersOut))
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	if len(orphans) == 0 {
		result.Success = true
		return result
	}

	result.ExitCode = 1
	result.Output = "refusing to update because existing containers use Compose service identities that are absent from the current compose file:\n" +
		strings.Join(orphans, "\n") +
		"\nReview the compose change and explicitly migrate, remove, or restore these services before retrying. Stack Manager will not remove orphans automatically.\n"
	return result
}

func commandExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

func findOrphanComposeServices(configured []string, containerRecords string) []string {
	allowed := make(map[string]struct{}, len(configured))
	for _, service := range configured {
		if service = strings.TrimSpace(service); service != "" {
			allowed[service] = struct{}{}
		}
	}
	orphans := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(containerRecords), "\n") {
		parts := strings.SplitN(line, "|", 2)
		service := strings.TrimSpace(parts[0])
		if service == "" {
			continue
		}
		if _, ok := allowed[service]; ok {
			continue
		}
		container := "unknown container"
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			container = strings.TrimSpace(parts[1])
		}
		orphans[fmt.Sprintf("- service %q (container %q)", service, container)] = struct{}{}
	}
	items := make([]string, 0, len(orphans))
	for item := range orphans {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}

// DockerExec runs a docker command (not compose) and returns the result.
func DockerExec(args ...string) (*ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		return result, err
	}

	return result, nil
}
