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
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
)

// EnvSettingsHandler reads and writes select settings so admins can tune them
// from the dashboard without SSH access. Boot-level settings (ports, cache) live
// in .env because Docker Compose / the store read them before the app runs;
// runtime settings live in the DB (app_settings) so .env is only initial config.
type EnvSettingsHandler struct {
	envFile string
	store   *storage.Store
	// setAPIKey updates the live API key used by the auth middleware, so a roll
	// takes effect without a restart. Wired from main.go.
	setAPIKey func(string)
}

func NewEnvSettingsHandler(stateDir string, store *storage.Store) *EnvSettingsHandler {
	// The .env file is bind-mounted into the server container at /app/.env.
	envFile := "/app/.env"
	if f := os.Getenv("ENV_FILE"); f != "" {
		envFile = f
	}
	return &EnvSettingsHandler{envFile: envFile, store: store}
}

// SetAPIKeyUpdater wires the live-key setter used by RollAPIKey.
func (h *EnvSettingsHandler) SetAPIKeyUpdater(fn func(string)) { h.setAPIKey = fn }

func (h *EnvSettingsHandler) apiKeyIsSet(r *http.Request) bool {
	if h.store == nil {
		return false
	}
	k, ok := h.store.GetSettingString(r.Context(), "api_key")
	return ok && strings.TrimSpace(k) != ""
}

var editableKeys = map[string]struct{}{
	"WEB_HTTP_PORT":           {},
	"WEB_SSL_PORT":            {},
	"CACHE_TTL_SECONDS":       {},
	"METRICS_REFRESH_MINUTES": {},
	"WARM_CACHE_TTL_MINUTES":  {},
	"HOST_URL":                {},
	"EXTRA_DOCKER_ROOTS":      {},
	"TZ":                      {},
}

// dbBackedKeys are runtime settings stored in the DB (app_settings) rather than
// .env. The rest of editableKeys (ports, cache TTL) stay in .env because they're
// consumed before the app/store are up. .env still seeds these on first run.
var dbBackedKeys = map[string]struct{}{
	"HOST_URL":                {},
	"TZ":                      {},
	"EXTRA_DOCKER_ROOTS":      {},
	"METRICS_REFRESH_MINUTES": {},
	"WARM_CACHE_TTL_MINUTES":  {},
	"CACHE_TTL_SECONDS":       {},
	"DOCKER_DAEMON_DIR":       {},
}

// settingValueOr returns a setting's effective value: DB (for db-backed keys)
// falling back to .env, else the .env value, else the default.
func (h *EnvSettingsHandler) settingValueOr(r *http.Request, envValues map[string]string, key, def string) string {
	envVal := valueOr(envValues, key, def)
	if _, db := dbBackedKeys[key]; db && h.store != nil {
		return h.store.SettingStringOr(r.Context(), key, envVal)
	}
	return envVal
}

type envSettings struct {
	WebHTTPPort       string `json:"web_http_port"`
	WebSSLPort        string `json:"web_ssl_port"`
	CacheTTLSeconds   string `json:"cache_ttl_seconds"`
	MetricsRefreshMin string `json:"metrics_refresh_minutes"`
	WarmCacheTTLMin   string `json:"warm_cache_ttl_minutes"`
	HostURL           string `json:"host_url"`
	ExtraDockerRoots  string `json:"extra_docker_roots"`
	Timezone          string `json:"tz"`
	DockerDaemonDir   string `json:"docker_daemon_dir"`
	APIKeySet         bool   `json:"api_key_set"`
}

func (h *EnvSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	values := readEnvFile(h.envFile)
	settings := envSettings{
		WebHTTPPort:       valueOr(values, "WEB_HTTP_PORT", "8193"),
		WebSSLPort:        valueOr(values, "WEB_SSL_PORT", "8993"),
		CacheTTLSeconds:   h.settingValueOr(r, values, "CACHE_TTL_SECONDS", "15"),
		MetricsRefreshMin: h.settingValueOr(r, values, "METRICS_REFRESH_MINUTES", "15"),
		WarmCacheTTLMin:   h.settingValueOr(r, values, "WARM_CACHE_TTL_MINUTES", "30"),
		HostURL:           h.settingValueOr(r, values, "HOST_URL", ""),
		ExtraDockerRoots:  h.settingValueOr(r, values, "EXTRA_DOCKER_ROOTS", ""),
		Timezone:          h.settingValueOr(r, values, "TZ", "UTC"),
		DockerDaemonDir:   h.settingValueOr(r, values, "DOCKER_DAEMON_DIR", "/etc/docker"),
		APIKeySet:         h.apiKeyIsSet(r) || values["API_KEY"] != "",
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

	// Route each key: DB-backed runtime settings go to app_settings; boot
	// settings (ports, cache) stay in .env. .env keeps seeding the DB-backed
	// ones on a fresh install because settingValueOr falls back to it.
	envChanged := map[string]string{}
	var portChanged, dbChanged bool
	for key, value := range changed {
		if _, db := dbBackedKeys[key]; db && h.store != nil {
			if err := h.store.SetSettingString(r.Context(), key, value); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			// Let readers that consult the environment at runtime (HOST_URL, TZ)
			// pick up the change without a restart.
			_ = os.Setenv(key, value)
			// The Redis cache TTL is a live field on the store — update it now.
			if key == "CACHE_TTL_SECONDS" {
				if n, err := strconv.Atoi(value); err == nil && n >= 1 {
					h.store.CacheTTL = time.Duration(n) * time.Second
				}
			}
			dbChanged = true
			continue
		}
		envChanged[key] = value
		if key == "WEB_HTTP_PORT" || key == "WEB_SSL_PORT" {
			portChanged = true
		}
	}

	if len(envChanged) > 0 {
		if err := updateEnvFile(h.envFile, envChanged); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if portChanged {
		warnings = append(warnings, "Port changes require a full stack restart: docker compose --env-file .env up -d")
	}
	if dbChanged {
		warnings = append(warnings, "Saved to the database. Metrics/cache/roots/timezone changes take full effect on the next restart.")
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
	newKey, err := GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate key: "+err.Error())
		return
	}
	if h.store == nil {
		writeError(w, http.StatusInternalServerError, "settings store unavailable")
		return
	}
	if err := h.store.SetSettingString(r.Context(), "api_key", newKey); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Take effect immediately.
	if h.setAPIKey != nil {
		h.setAPIKey(newKey)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"api_key": newKey,
		"warning": "New API key is active now and shown once — save it. The previous key stops working immediately. Session logins are unaffected.",
	})
}

// GenerateAPIKey returns a new random API key.
func GenerateAPIKey() (string, error) {
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
