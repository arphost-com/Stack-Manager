package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DockerSettingsHandler struct {
	DaemonDir       string
	BaseImagePrefix string
}

type dockerDaemonSaveRequest struct {
	Config map[string]interface{} `json:"config"`
	Raw    string                 `json:"raw,omitempty"`
}

type dockerDaemonResponse struct {
	Path            string                 `json:"path"`
	Exists          bool                   `json:"exists"`
	Config          map[string]interface{} `json:"config"`
	Raw             string                 `json:"raw"`
	Backup          string                 `json:"backup,omitempty"`
	RestartRequired bool                   `json:"restart_required"`
	Warnings        []string               `json:"warnings,omitempty"`
}

func NewDockerSettingsHandler(daemonDir, baseImagePrefix string) *DockerSettingsHandler {
	if strings.TrimSpace(daemonDir) == "" {
		daemonDir = "/etc/docker"
	}
	return &DockerSettingsHandler{DaemonDir: daemonDir, BaseImagePrefix: baseImagePrefix}
}

func (h *DockerSettingsHandler) helperImage() string {
	return h.BaseImagePrefix + "alpine:3.22"
}

func (h *DockerSettingsHandler) GetDaemon(w http.ResponseWriter, r *http.Request) {
	raw, exists, err := h.readDaemonJSON()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	config := map[string]interface{}{}
	if exists && strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			writeError(w, http.StatusInternalServerError, "daemon.json is not valid JSON: "+err.Error())
			return
		}
	}
	pretty := prettyJSON(config)
	writeJSON(w, http.StatusOK, dockerDaemonResponse{
		Path:     filepath.Join(h.hostDaemonDir(), "daemon.json"),
		Exists:   exists,
		Config:   config,
		Raw:      pretty,
		Warnings: dockerDaemonWarnings(config),
	})
}

func (h *DockerSettingsHandler) SaveDaemon(w http.ResponseWriter, r *http.Request) {
	var req dockerDaemonSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config := req.Config
	if strings.TrimSpace(req.Raw) != "" {
		if err := json.Unmarshal([]byte(req.Raw), &config); err != nil {
			writeError(w, http.StatusBadRequest, "daemon JSON is invalid: "+err.Error())
			return
		}
	}
	if config == nil {
		config = map[string]interface{}{}
	}
	raw := prettyJSON(config)
	backup, _ := h.backupDaemonJSON()
	if err := h.writeDaemonJSON(raw); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dockerDaemonResponse{
		Path:            filepath.Join(h.hostDaemonDir(), "daemon.json"),
		Exists:          true,
		Config:          config,
		Raw:             raw,
		Backup:          backup,
		RestartRequired: true,
		Warnings:        dockerDaemonWarnings(config),
	})
}

func (h *DockerSettingsHandler) readDaemonJSON() (string, bool, error) {
	output, err := h.runDocker("run", "--rm", "-v", h.hostDaemonDir()+":/host/etc/docker:ro", h.helperImage(), "cat", "/host/etc/docker/daemon.json")
	if err != nil {
		text := strings.TrimSpace(string(output))
		if strings.Contains(text, "No such file") || strings.Contains(text, "can't open") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read daemon.json failed: %v - %s", err, text)
	}
	return string(output), true, nil
}

func (h *DockerSettingsHandler) writeDaemonJSON(raw string) error {
	args := []string{"run", "--rm", "-i", "-v", h.hostDaemonDir() + ":/host/etc/docker", h.helperImage(), "tee", "/host/etc/docker/daemon.json"}
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(raw)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write daemon.json failed: %v - %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (h *DockerSettingsHandler) backupDaemonJSON() (string, error) {
	backupName := "daemon.json.stack-manager-" + time.Now().UTC().Format("20060102_150405") + ".bak"
	output, err := h.runDocker("run", "--rm", "-v", h.hostDaemonDir()+":/host/etc/docker", h.helperImage(), "cp", "/host/etc/docker/daemon.json", "/host/etc/docker/"+backupName)
	if err != nil {
		return "", fmt.Errorf("backup daemon.json failed: %v - %s", err, strings.TrimSpace(string(output)))
	}
	return filepath.Join(h.hostDaemonDir(), backupName), nil
}

func (h *DockerSettingsHandler) hostDaemonDir() string {
	dir := strings.TrimSpace(h.DaemonDir)
	if dir == "" {
		return "/etc/docker"
	}
	return filepath.Clean(dir)
}

func (h *DockerSettingsHandler) runDocker(args ...string) ([]byte, error) {
	dir := h.hostDaemonDir()
	if !strings.HasPrefix(dir, "/") || strings.ContainsAny(dir, "\r\n") {
		return nil, fmt.Errorf("DOCKER_DAEMON_DIR must be an absolute host path")
	}
	return exec.Command("docker", args...).CombinedOutput()
}

func prettyJSON(config map[string]interface{}) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return "{}\n"
	}
	return out.String()
}

func dockerDaemonWarnings(config map[string]interface{}) []string {
	warnings := []string{}
	hosts, _ := config["hosts"].([]interface{})
	for _, item := range hosts {
		host, ok := item.(string)
		if !ok || !strings.HasPrefix(host, "tcp://") {
			continue
		}
		if strings.Contains(host, "0.0.0.0:2375") || strings.Contains(host, ":2375") {
			warnings = append(warnings, "Remote Docker TCP on port 2375 is usually unauthenticated root-equivalent access. Prefer SSH, TLS on 2376, VPN-only binding, or a firewall allowlist.")
		} else {
			warnings = append(warnings, "Remote Docker TCP exposes root-equivalent Docker API access. Restrict the bind address and require TLS where possible.")
		}
	}
	if insecure, ok := config["insecure-registries"].([]interface{}); ok && len(insecure) > 0 {
		warnings = append(warnings, "Insecure registries disable TLS verification for matching registries. Use only on trusted private networks.")
	}
	return warnings
}
