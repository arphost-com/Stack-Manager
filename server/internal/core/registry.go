package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// RegistryLogin is a redacted view of one saved docker login for the UI.
type RegistryLogin struct {
	Registry     string `json:"registry"`      // "https://index.docker.io/v1/" for Docker Hub, host[:port] for others
	Display      string `json:"display"`       // "Docker Hub" or the registry host
	Username     string `json:"username"`      // full username, safe to show
	MaskedUser   string `json:"masked_user"`   // partially starred version for display
	HasPassword  bool   `json:"has_password"`  // always true if the entry exists
	StoredIn     string `json:"stored_in"`     // path to the docker config that holds this auth
}

// ListSavedRegistryLogins reads the docker config from DOCKER_CONFIG (or the
// default location) and returns each saved login with the password redacted.
func ListSavedRegistryLogins() ([]RegistryLogin, error) {
	path := resolveDockerConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RegistryLogin{}, nil
		}
		return nil, err
	}
	var cfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	out := make([]RegistryLogin, 0, len(cfg.Auths))
	for registry, entry := range cfg.Auths {
		user := ""
		if entry.Auth != "" {
			if decoded, err := base64.StdEncoding.DecodeString(entry.Auth); err == nil {
				if idx := strings.IndexByte(string(decoded), ':'); idx >= 0 {
					user = string(decoded[:idx])
				}
			}
		}
		out = append(out, RegistryLogin{
			Registry:    registry,
			Display:     displayNameForRegistry(registry),
			Username:    user,
			MaskedUser:  maskUsername(user),
			HasPassword: entry.Auth != "",
			StoredIn:    path,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Display < out[j].Display })
	return out, nil
}

// DeleteSavedRegistryLogin removes a saved login from the docker config. Uses
// `docker logout` when possible so docker handles credential helpers correctly,
// falling back to a direct JSON edit if the CLI call fails.
func DeleteSavedRegistryLogin(registry string) *OpResult {
	result := &OpResult{Project: "(registry)", Action: "logout"}
	if !validRegistryName(strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://")) && registry != "https://index.docker.io/v1/" {
		result.Success = false
		result.ExitCode = 1
		result.Output = "invalid registry name"
		return result
	}
	args := []string{"logout"}
	if registry != "" && registry != "https://index.docker.io/v1/" {
		args = append(args, registry)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Fallback: edit the config file directly.
		if err := removeAuthFromConfig(registry); err != nil {
			result.Success = false
			result.ExitCode = 1
			result.Output = strings.TrimSpace(stdout.String()+stderr.String()) + "\nfallback edit failed: " + err.Error()
			return result
		}
	}
	result.Success = true
	result.ExitCode = 0
	result.Output = strings.TrimSpace(stdout.String() + stderr.String())
	if registry == "https://index.docker.io/v1/" || registry == "" {
		result.Action = "logout docker-hub"
	} else {
		result.Action = "logout " + registry
	}
	return result
}

func resolveDockerConfigPath() string {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/root/.docker/config.json"
	}
	return filepath.Join(home, ".docker", "config.json")
}

func removeAuthFromConfig(registry string) error {
	path := resolveDockerConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	if auths, ok := cfg["auths"].(map[string]interface{}); ok {
		delete(auths, registry)
		cfg["auths"] = auths
	}
	out, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func displayNameForRegistry(registry string) string {
	if registry == "" || registry == "https://index.docker.io/v1/" {
		return "Docker Hub"
	}
	return registry
}

func maskUsername(user string) string {
	if user == "" {
		return ""
	}
	runes := []rune(user)
	if len(runes) <= 3 {
		return string(runes[:1]) + strings.Repeat("*", len(runes)-1)
	}
	return string(runes[:2]) + strings.Repeat("*", len(runes)-3) + string(runes[len(runes)-1:])
}

// DockerLogin authenticates Docker against a registry using password-stdin.
func DockerLogin(req RegistryLoginRequest) *OpResult {
	registry := strings.TrimSpace(req.Registry)
	username := strings.TrimSpace(req.Username)

	result := &OpResult{
		Project: "(registry)",
		Action:  "login",
	}

	if username == "" || req.Password == "" {
		result.Success = false
		result.ExitCode = 1
		result.Output = "username and password are required"
		return result
	}
	if !validRegistryName(registry) {
		result.Success = false
		result.ExitCode = 1
		result.Output = "registry must be a Docker registry host without a URL scheme or path"
		return result
	}

	args := []string{"login"}
	if registry != "" {
		args = append(args, registry)
	}
	args = append(args, "-u", username, "--password-stdin")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(req.Password)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	result.Output = sanitizeLoginOutput(stdout.String() + stderr.String())
	if registry != "" {
		result.Action = fmt.Sprintf("login %s", registry)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Success = false
		return result
	}

	result.ExitCode = 0
	result.Success = true
	return result
}

func validRegistryName(registry string) bool {
	if registry == "" {
		return true
	}
	if strings.Contains(registry, "://") || strings.Contains(registry, "/") {
		return false
	}
	for _, r := range registry {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func sanitizeLoginOutput(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "password") {
			lines[i] = strings.ReplaceAll(line, "\r", "")
		}
	}
	return strings.Join(lines, "\n")
}
