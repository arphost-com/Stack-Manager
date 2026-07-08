package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var validProjectDirName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// CreateProject creates a compose project under the engine root.
func (e *Engine) CreateProject(req CreateProjectRequest) (*Project, error) {
	name := strings.TrimSpace(req.Name)
	if !validProjectDirName.MatchString(name) {
		return nil, fmt.Errorf("project name must start with a letter or number and contain only letters, numbers, dots, underscores, or hyphens")
	}
	if strings.TrimSpace(req.ComposeContent) == "" {
		return nil, fmt.Errorf("compose content is required")
	}

	rootAbs, err := filepath.Abs(e.RootDir)
	if err != nil {
		return nil, err
	}
	projectDir := filepath.Join(rootAbs, name)
	projectAbs, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	if projectAbs != filepath.Join(rootAbs, name) {
		return nil, fmt.Errorf("invalid project path")
	}

	if _, err := os.Stat(projectDir); err == nil && !req.Overwrite {
		return nil, fmt.Errorf("project already exists: %s", name)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(projectDir, 0750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "compose.yml"), []byte(req.ComposeContent), 0640); err != nil {
		return nil, err
	}
	uid, gid, err := resolveCreateRunUser(req)
	if err != nil {
		return nil, err
	}
	envContent := mergeRunUserEnv(req.EnvContent, uid, gid)
	if envContent != "" {
		if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0600); err != nil {
			return nil, err
		}
	}
	if shouldEnforceCreateUser(req) {
		if err := writeComposeUserOverride(projectDir, filepath.Join(projectDir, "compose.yml"), uid, gid); err != nil {
			return nil, err
		}
	}
	if req.Inactive {
		if err := os.WriteFile(filepath.Join(projectDir, inactiveMarker), []byte{}, 0640); err != nil {
			return nil, err
		}
	}

	cf := composeFileForDir(projectDir)
	if cf == "" {
		return nil, fmt.Errorf("project was created but no compose file could be found")
	}
	project := e.buildProject(projectDir, cf)
	return &project, nil
}

// shouldEnforceCreateUser returns true when the Create Project request
// explicitly asks Stack Manager to inject a user: line into every
// service via a generated compose.override.yml.
//
// Historically this defaulted to true. That silently broke templates
// whose images have a required internal UID — MariaDB / MySQL run as
// UID 999, Postgres runs as UID 999, and WordPress's first-boot tar
// copy must be root. Forcing SERVER_USER (typically 1000) on those
// services produced "Errcode: 13 Permission denied" and
// "Operation not permitted" crashes at first up.
//
// New default: only enforce when the caller sets EnforceUser=true. Bind
// mounts that need SERVER_USER ownership are the rare case and should
// be opted into per project. This matches how docker compose itself
// behaves out of the box: images run as whatever user they were built
// for.
func shouldEnforceCreateUser(req CreateProjectRequest) bool {
	return req.EnforceUser != nil && *req.EnforceUser
}

func resolveCreateRunUser(req CreateProjectRequest) (string, string, error) {
	uid := strings.TrimSpace(req.RunAsUID)
	gid := strings.TrimSpace(req.RunAsGID)
	if uid == "" {
		uid = strconv.Itoa(os.Getuid())
	}
	if gid == "" {
		gid = strconv.Itoa(os.Getgid())
	}
	if !numericID(uid) || !numericID(gid) {
		return "", "", fmt.Errorf("run-as UID and GID must be numeric")
	}
	return uid, gid, nil
}

func numericID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func mergeRunUserEnv(existing, uid, gid string) string {
	values := map[string]string{
		"STACK_UID": uid,
		"STACK_GID": gid,
		"PUID":      uid,
		"PGID":      gid,
		"UID":       uid,
		"GID":       gid,
		"USER_UID":  uid,
		"USER_GID":  gid,
	}
	present := readEnvContentKeys(existing)
	var out strings.Builder
	trimmed := strings.TrimRight(existing, "\n")
	if trimmed != "" {
		out.WriteString(trimmed)
		out.WriteString("\n")
	}
	for _, key := range composeUserEnvKeys {
		if _, ok := present[key]; ok {
			continue
		}
		out.WriteString(key)
		out.WriteString("=")
		out.WriteString(values[key])
		out.WriteString("\n")
	}
	return out.String()
}

func readEnvContentKeys(content string) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		keys[strings.TrimSpace(line[:idx])] = struct{}{}
	}
	return keys
}

func writeComposeUserOverride(projectDir, composeFile, uid, gid string) error {
	services, err := composeConfigServices(projectDir, composeFile, uid, gid)
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return nil
	}
	var out strings.Builder
	out.WriteString("# Generated by Stack Manager. Edit STACK_UID/STACK_GID in .env or this file to run a stack as a different user.\n")
	out.WriteString("services:\n")
	for _, service := range services {
		out.WriteString("  ")
		out.WriteString(service)
		out.WriteString(":\n")
		out.WriteString("    user: \"${STACK_UID:-")
		out.WriteString(uid)
		out.WriteString("}:${STACK_GID:-")
		out.WriteString(gid)
		out.WriteString("}\"\n")
	}
	return os.WriteFile(filepath.Join(projectDir, "compose.override.yml"), []byte(out.String()), 0640)
}

func composeConfigServices(projectDir, composeFile, uid, gid string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "config", "--services")
	cmd.Dir = projectDir
	cmd.Env = append(cmd.Environ(),
		"STACK_UID="+uid,
		"STACK_GID="+gid,
		"PUID="+uid,
		"PGID="+gid,
		"UID="+uid,
		"GID="+gid,
		"USER_UID="+uid,
		"USER_GID="+gid,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("compose service discovery failed: %s", msg)
	}
	var services []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		service := strings.TrimSpace(line)
		if service == "" {
			continue
		}
		if !validComposeServiceName(service) {
			return nil, fmt.Errorf("invalid compose service name: %s", service)
		}
		services = append(services, service)
	}
	return services, nil
}

func validComposeServiceName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}
