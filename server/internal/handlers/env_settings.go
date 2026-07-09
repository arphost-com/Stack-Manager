package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
)

// EnvSettingsHandler reads and writes select .env values so admins
// can tune ports, cache TTLs, and API keys from the dashboard without
// SSH access. Only whitelisted keys are exposed.
type EnvSettingsHandler struct {
	envFile string
}

func NewEnvSettingsHandler(stateDir string) *EnvSettingsHandler {
	// The .env file is bind-mounted into the server container at /app/.env.
	envFile := "/app/.env"
	if f := os.Getenv("ENV_FILE"); f != "" {
		envFile = f
	}
	return &EnvSettingsHandler{envFile: envFile}
}

var editableKeys = map[string]struct{}{
	"WEB_HTTP_PORT":          {},
	"WEB_SSL_PORT":           {},
	"CACHE_TTL_SECONDS":      {},
	"METRICS_REFRESH_MINUTES": {},
	"WARM_CACHE_TTL_MINUTES": {},
	"HOST_URL":               {},
	"EXTRA_DOCKER_ROOTS":     {},
}

type envSettings struct {
	WebHTTPPort          string `json:"web_http_port"`
	WebSSLPort           string `json:"web_ssl_port"`
	CacheTTLSeconds      string `json:"cache_ttl_seconds"`
	MetricsRefreshMin    string `json:"metrics_refresh_minutes"`
	WarmCacheTTLMin      string `json:"warm_cache_ttl_minutes"`
	HostURL              string `json:"host_url"`
	ExtraDockerRoots     string `json:"extra_docker_roots"`
	APIKeySet            bool   `json:"api_key_set"`
}

func (h *EnvSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	values := readEnvFile(h.envFile)
	settings := envSettings{
		WebHTTPPort:       valueOr(values, "WEB_HTTP_PORT", "8193"),
		WebSSLPort:        valueOr(values, "WEB_SSL_PORT", "8993"),
		CacheTTLSeconds:   valueOr(values, "CACHE_TTL_SECONDS", "15"),
		MetricsRefreshMin: valueOr(values, "METRICS_REFRESH_MINUTES", "15"),
		WarmCacheTTLMin:   valueOr(values, "WARM_CACHE_TTL_MINUTES", "30"),
		HostURL:           valueOr(values, "HOST_URL", ""),
		ExtraDockerRoots:  valueOr(values, "EXTRA_DOCKER_ROOTS", ""),
		APIKeySet:         values["API_KEY"] != "",
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *EnvSettingsHandler) Save(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var warnings []string
	changed := map[string]string{}
	for key, value := range body {
		upper := strings.ToUpper(strings.TrimSpace(key))
		if _, ok := editableKeys[upper]; !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if upper == "WEB_HTTP_PORT" || upper == "WEB_SSL_PORT" {
			if _, err := strconv.Atoi(value); err != nil && value != "0" {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("%s must be a number", upper))
				return
			}
		}
		if upper == "CACHE_TTL_SECONDS" || upper == "METRICS_REFRESH_MINUTES" || upper == "WARM_CACHE_TTL_MINUTES" {
			if n, err := strconv.Atoi(value); err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("%s must be a positive number", upper))
				return
			}
		}
		changed[upper] = value
	}

	if len(changed) == 0 {
		writeError(w, http.StatusBadRequest, "no valid settings to update")
		return
	}

	if err := updateEnvFile(h.envFile, changed); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for key := range changed {
		if key == "WEB_HTTP_PORT" || key == "WEB_SSL_PORT" {
			warnings = append(warnings, "Port changes require a full stack restart: docker compose --env-file .env up -d")
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"saved":    changed,
		"warnings": warnings,
	})
}

func (h *EnvSettingsHandler) RollAPIKey(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	newKey, err := generateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate key: "+err.Error())
		return
	}
	if err := updateEnvFile(h.envFile, map[string]string{"API_KEY": newKey}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"api_key": newKey,
		"warning": "The new API key is saved to .env and will take effect on the next server restart. Existing API-key sessions will stop working. Session-based logins are not affected.",
	})
}

func generateAPIKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]) + "._%-", nil
}

var envLineRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

func readEnvFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := envLineRe.FindStringSubmatch(line)
		if m != nil {
			values[m[1]] = m[2]
		}
	}
	return values
}

func updateEnvFile(path string, updates map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	touched := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := envLineRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		key := m[1]
		if newVal, ok := updates[key]; ok {
			lines[i] = key + "=" + newVal
			touched[key] = true
		}
	}
	for key, val := range updates {
		if !touched[key] {
			lines = append(lines, key+"="+val)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

func valueOr(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return fallback
}
