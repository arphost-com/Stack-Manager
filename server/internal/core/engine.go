package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const inactiveMarker = ".inactive"

// ErrNotFound is returned when a project or resource is not found.
type ErrNotFound struct {
	Msg string
}

func (e *ErrNotFound) Error() string { return e.Msg }

// Engine is the core compose management engine.
type Engine struct {
	RootDir  string
	HooksDir string
}

// NewEngine creates a new Engine.
func NewEngine(rootDir, hooksDir string) *Engine {
	return &Engine{
		RootDir:  rootDir,
		HooksDir: hooksDir,
	}
}

// GetProject returns a single project by name.
func (e *Engine) GetProject(name string) (*Project, error) {
	projects, err := e.DiscoverProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, &ErrNotFound{Msg: "project not found: " + name}
}

// Pull pulls images for a project.
func (e *Engine) Pull(project *Project, timeout int) *OpResult {
	return e.ExecComposeWithTimeout(project, timeout, "pull")
}

// Up brings up containers for a project.
func (e *Engine) Up(project *Project) *OpResult {
	return e.ExecCompose(project, "up", "-d")
}

// Down stops and removes containers for a project.
func (e *Engine) Down(project *Project) *OpResult {
	return e.ExecCompose(project, "down")
}

// Restart restarts containers for a project.
func (e *Engine) Restart(project *Project) *OpResult {
	return e.ExecCompose(project, "restart")
}

// Update performs a full update: if a post-update hook exists, run only that;
// otherwise pull + up.
func (e *Engine) Update(project *Project, timeout int) []OpResult {
	if e.HasHook("post", "update", project.Name) {
		hookResult := e.RunHook("post", "update", project)
		hookResult.Action = "update (hook)"
		return []OpResult{*hookResult}
	}

	var results []OpResult
	pullResult := e.Pull(project, timeout)
	pullResult.Action = "pull"
	results = append(results, *pullResult)

	if pullResult.Success {
		upResult := e.Up(project)
		upResult.Action = "up"
		results = append(results, *upResult)
	}

	return results
}

// Status returns the compose ps output for a project.
func (e *Engine) Status(project *Project) *OpResult {
	return e.ExecCompose(project, "ps")
}

// SetInactive marks or unmarks a project as inactive.
func (e *Engine) SetInactive(name string, inactive bool) error {
	project, err := e.GetProject(name)
	if err != nil {
		return err
	}

	markerPath := filepath.Join(project.Dir, inactiveMarker)

	if inactive {
		f, err := os.Create(markerPath)
		if err != nil {
			return fmt.Errorf("failed to create inactive marker: %w", err)
		}
		f.Close()
	} else {
		if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove inactive marker: %w", err)
		}
	}

	return nil
}

// DeleteProject stops a compose project and removes its directory.
func (e *Engine) DeleteProject(name string, req DeleteProjectRequest) (*OpResult, error) {
	project, err := e.GetProject(name)
	if err != nil {
		return nil, err
	}
	if req.ConfirmName != project.Name {
		return nil, fmt.Errorf("confirmation must exactly match project name")
	}
	if !project.Inactive {
		return nil, fmt.Errorf("project must be marked inactive before it can be deleted")
	}
	rootAbs, err := filepath.Abs(e.RootDir)
	if err != nil {
		return nil, err
	}
	projectAbs, err := filepath.Abs(project.Dir)
	if err != nil {
		return nil, err
	}
	if projectAbs == rootAbs {
		return nil, fmt.Errorf("refusing to delete the configured Docker root")
	}
	rel, err := filepath.Rel(rootAbs, projectAbs)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return nil, fmt.Errorf("project directory is outside the configured Docker root")
	}
	if strings.Contains(rel, string(os.PathSeparator)) {
		return nil, fmt.Errorf("project directory must be an immediate child of the configured Docker root")
	}

	result := &OpResult{
		Project: project.Name,
		Action:  "delete",
		Success: true,
	}
	if req.StopFirst {
		down := e.Down(project)
		result.Output += "=== docker compose down ===\n" + down.Output + "\n"
		if !down.Success {
			result.Success = false
			result.ExitCode = down.ExitCode
			return result, fmt.Errorf("compose down failed")
		}
	}
	if err := os.RemoveAll(projectAbs); err != nil {
		result.Success = false
		result.ExitCode = 1
		result.Output += err.Error() + "\n"
		return result, err
	}
	result.Output += "deleted directory: " + projectAbs + "\n"
	return result, nil
}

// Prune runs a selected Docker prune command.
func (e *Engine) Prune(mode string) *OpResult {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = "safe"
	}
	result := &OpResult{
		Project: "(system)",
		Action:  "prune:" + mode,
	}

	var outputs []string
	run := func(label string, args ...string) bool {
		cmdResult, _ := DockerExec(args...)
		if cmdResult == nil {
			outputs = append(outputs, "=== "+label+" ===\nfailed to run command")
			return false
		}
		outputs = append(outputs, "=== "+label+" ===\n"+cmdResult.Stdout+cmdResult.Stderr)
		if cmdResult.ExitCode != 0 {
			result.ExitCode = cmdResult.ExitCode
			return false
		}
		return true
	}
	success := true
	switch mode {
	case "safe":
		success = run("Image Prune", "image", "prune", "-f") && success
		success = run("Network Prune", "network", "prune", "-f") && success
		success = run("Volume Prune", "volume", "prune", "-f") && success
	case "system":
		success = run("System Prune", "system", "prune", "-f")
	case "system-all":
		success = run("System Prune All", "system", "prune", "--all", "--volumes", "-f")
	case "images":
		success = run("Image Prune All", "image", "prune", "--all", "-f")
	case "volumes":
		success = run("Volume Prune", "volume", "prune", "-f")
	case "networks":
		success = run("Network Prune", "network", "prune", "-f")
	case "builder":
		success = run("Builder Prune", "builder", "prune", "--all", "-f")
	default:
		result.Success = false
		result.ExitCode = 1
		result.Output = "unsupported prune mode: " + mode + "\n"
		return result
	}
	result.Output = joinOutputs(outputs)
	result.Success = success
	return result
}

func joinOutputs(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p + "\n"
	}
	return out
}
