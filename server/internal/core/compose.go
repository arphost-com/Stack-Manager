package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// getProjectName returns the compose project name, preferring a running label.
func (e *Engine) getProjectName(name string) string {
	// Try to detect from running containers first
	if label := e.detectRunningLabel(name); label != "" {
		return label
	}
	return sanitizeProjectName(name)
}

// detectRunningLabel checks if containers are running with a compose project label.
func (e *Engine) detectRunningLabel(name string) string {
	sanitized := sanitizeProjectName(name)
	candidates := []string{sanitized, strings.ToLower(name)}

	for _, cand := range candidates {
		out, err := exec.Command("docker", "ps",
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

// getContainers returns the running containers for a project.
func (e *Engine) getContainers(name string) ([]Container, bool) {
	pname := e.getProjectName(name)
	out, err := exec.Command("docker", "ps",
		"--filter", fmt.Sprintf("label=com.docker.compose.project=%s", pname),
		"--format", `{{json .}}`,
	).Output()
	if err != nil {
		return nil, false
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

	return containers, len(containers) > 0
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
