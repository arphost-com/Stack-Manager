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
	RootDir    string
	ExtraRoots []string
	HooksDir   string
	dockerExec func(args ...string) (*ExecResult, error)
	hostname   func() (string, error)
}

// NewEngine creates a new Engine.
func NewEngine(rootDir, hooksDir string) *Engine {
	return &Engine{
		RootDir:    rootDir,
		HooksDir:   hooksDir,
		dockerExec: DockerExec,
		hostname:   os.Hostname,
	}
}

// AllRoots returns the primary root followed by any extra roots.
func (e *Engine) AllRoots() []string {
	roots := []string{e.RootDir}
	for _, r := range e.ExtraRoots {
		r = strings.TrimSpace(r)
		if r != "" && r != e.RootDir {
			roots = append(roots, r)
		}
	}
	return roots
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
	if e.IsControllerProject(project) {
		return controllerImageActionResult(project, "pull")
	}
	return e.ExecComposeWithTimeout(project, timeout, composePullArgs(project)...)
}

// composePullArgs keeps registry-backed services updatable when a Compose
// project also contains locally built services. Without --ignore-buildable,
// Docker Compose attempts to pull the local image name (for example
// arpvpn:local) and turns an otherwise valid update into a pull failure.
func composePullArgs(project *Project) []string {
	args := []string{"pull"}
	for _, source := range project.ImageSources {
		if source.Build {
			return append(args, "--ignore-buildable")
		}
	}
	return args
}

// Up brings up containers for a project.
func (e *Engine) Up(project *Project) *OpResult {
	if preflight := e.CheckComposeServiceIdentity(project); !preflight.Success {
		return preflight
	}
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
	if e.IsControllerProject(project) {
		return []OpResult{*controllerImageActionResult(project, "update")}
	}
	if e.HasHook("post", "update", project.Name) {
		hookResult := e.RunHook("post", "update", project)
		hookResult.Action = "update (hook)"
		return []OpResult{*hookResult}
	}
	if preflight := e.CheckComposeServiceIdentity(project); !preflight.Success {
		return []OpResult{*preflight}
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
	projectAbs, err := filepath.Abs(project.Dir)
	if err != nil {
		return nil, err
	}
	var containingRoot string
	var projectRel string
	for _, root := range e.AllRoots() {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if projectAbs == rootAbs {
			return nil, fmt.Errorf("refusing to delete the configured Docker root")
		}
		rel, err := filepath.Rel(rootAbs, projectAbs)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
			continue
		}
		if !strings.Contains(rel, string(os.PathSeparator)) {
			containingRoot = rootAbs
			projectRel = rel
			break
		}
	}
	if containingRoot == "" {
		return nil, fmt.Errorf("project directory is outside the configured Docker roots")
	}

	result := &OpResult{
		Project: project.Name,
		Action:  "delete",
		Success: true,
	}
	if req.StopFirst {
		// Skip compose down entirely if the project has no running
		// containers. This is the common case for stale directories left
		// over from a rename or import - the compose file often has
		// unresolvable ${REQUIRED} env vars because the sibling .env is
		// gone, and `docker compose down` fails at interpolation before
		// even checking that there is nothing to stop. Blocking Delete on
		// that failure meant operators could not remove a directory that
		// had no containers running anyway.
		containers, _ := e.getContainers(project.Name)
		if len(containers) > 0 {
			down := e.Down(project)
			result.Output += "=== docker compose down ===\n" + down.Output + "\n"
			if !down.Success {
				result.Success = false
				result.ExitCode = down.ExitCode
				return result, fmt.Errorf("compose down failed")
			}
		} else {
			result.Output += "=== docker compose down ===\nno running containers, skipping\n"
		}
	}
	if err := e.removeProjectDirectory(projectAbs, containingRoot, projectRel); err != nil {
		result.Success = false
		result.ExitCode = 1
		result.Output += err.Error() + "\n"
		return result, err
	}
	result.Output += "deleted directory: " + projectAbs + "\n"
	return result, nil
}

// removeProjectDirectory deletes a validated direct child of a configured
// Docker root. Production controllers run as an unprivileged host UID, while a
// managed container may legitimately create root-owned or service-owned files.
// The controller already has Docker-daemon authority, so use its current,
// locally available image as a short-lived root helper instead of making the
// long-running API process root. Only the validated Docker root is mounted and
// the helper has no network access.
func (e *Engine) removeProjectDirectory(projectAbs, rootAbs, projectRel string) error {
	dockerExec := e.dockerExec
	if dockerExec == nil {
		dockerExec = DockerExec
	}
	hostname := e.hostname
	if hostname == nil {
		hostname = os.Hostname
	}

	containerID, err := hostname()
	containerID = strings.TrimSpace(containerID)
	if err != nil || !looksLikeContainerID(containerID) {
		return os.RemoveAll(projectAbs)
	}

	inspect, inspectErr := dockerExec("inspect", "--format", "{{.Image}}", containerID)
	if inspectErr != nil {
		return fmt.Errorf("privileged deletion helper unavailable: cannot inspect controller container: %w", inspectErr)
	}
	if inspect == nil {
		return fmt.Errorf("privileged deletion helper unavailable: controller inspection returned no result")
	}
	imageID := strings.TrimSpace(inspect.Stdout)
	if !validDockerImageID(imageID) {
		return fmt.Errorf("privileged deletion helper unavailable: controller image ID was invalid")
	}

	target := filepath.Join("/managed-root", projectRel)
	remove, removeErr := dockerExec(
		"run", "--rm", "--pull=never", "--user", "0:0",
		"--network", "none", "--read-only", "--pids-limit", "64",
		"--cap-drop", "ALL", "--cap-add", "DAC_OVERRIDE", "--cap-add", "FOWNER",
		"--security-opt", "no-new-privileges",
		"--mount", "type=bind,source="+rootAbs+",target=/managed-root",
		"--entrypoint", "/bin/rm", imageID,
		"-rf", "--", target,
	)
	if removeErr != nil {
		return deletionHelperError(removeErr, remove)
	}
	if _, err := os.Lstat(projectAbs); err == nil {
		return fmt.Errorf("privileged deletion helper exited successfully but the project directory still exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot verify project directory removal: %w", err)
	}
	return nil
}

func deletionHelperError(runErr error, result *ExecResult) error {
	if result != nil {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		if detail != "" {
			return fmt.Errorf("privileged deletion helper failed: %s", detail)
		}
	}
	return fmt.Errorf("privileged deletion helper failed: %w", runErr)
}

func looksLikeContainerID(value string) bool {
	if len(value) < 12 || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func validDockerImageID(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	return looksLikeContainerID(strings.TrimPrefix(value, "sha256:"))
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
