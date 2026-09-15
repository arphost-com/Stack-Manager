package core

import (
	"os"
	"strings"
)

const ControllerImageActionMessage = "Stack Manager cannot update its own Compose project through stack actions; use Settings > Update"

func controllerImageActionResult(project *Project, action string) *OpResult {
	name := ""
	if project != nil {
		name = project.Name
	}
	return &OpResult{Project: name, Action: action, Success: false, Output: ControllerImageActionMessage + "\n", ExitCode: -1}
}

// IsControllerProject identifies the Compose project running this process.
// Docker containers use their container ID as the default hostname, and
// discovery already returns those IDs. The name fallback protects standard
// installs even when a custom hostname hides the ID.
func (e *Engine) IsControllerProject(project *Project) bool {
	if project == nil {
		return false
	}
	if configured := strings.TrimSpace(os.Getenv("STACK_MANAGER_PROJECT_NAME")); configured != "" {
		if project.Name == configured {
			return true
		}
	}
	hostname, _ := os.Hostname()
	if projectContainsContainerID(*project, hostname) {
		return true
	}
	return project.Name == "stack-manager"
}

func projectContainsContainerID(project Project, candidate string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if len(candidate) < 12 || !isHex(candidate) {
		return false
	}
	for _, container := range project.Containers {
		id := strings.ToLower(strings.TrimSpace(container.ID))
		if len(id) >= 12 && isHex(id) && (strings.HasPrefix(id, candidate) || strings.HasPrefix(candidate, id)) {
			return true
		}
	}
	return false
}

func isHex(value string) bool {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return value != ""
}
