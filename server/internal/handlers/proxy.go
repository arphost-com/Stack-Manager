package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
)

const npmSettingKey = "npm_connection"

type npmPersisted struct {
	URL      string `json:"url"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ProxyHandler manages communication with a Nginx Proxy Manager
// instance. It stores the NPM base URL and credentials in memory
// (set from the Settings UI) and proxies API calls so the dashboard
// can list, create, and delete proxy hosts without the user touching
// NPM's admin panel directly.
type ProxyHandler struct {
	engine *core.Engine
	store  *storage.Store

	mu       sync.RWMutex
	npmURL   string
	npmToken string
	npmExp   time.Time
	npmUser  string
	npmPass  string
}

func NewProxyHandler(engine *core.Engine, store *storage.Store) *ProxyHandler {
	h := &ProxyHandler{engine: engine, store: store}
	h.loadPersisted()
	return h
}

// loadPersisted restores the NPM connection (URL + credentials) saved by a
// previous Configure call so the reverse-proxy connection survives restarts.
// The bearer token is not persisted; ensureToken re-authenticates lazily.
func (h *ProxyHandler) loadPersisted() {
	if h.store == nil {
		return
	}
	raw, err := h.store.GetSetting(context.Background(), npmSettingKey)
	if err != nil || raw == "" {
		return
	}
	var p npmPersisted
	if json.Unmarshal([]byte(raw), &p) != nil || p.URL == "" {
		return
	}
	h.mu.Lock()
	h.npmURL = p.URL
	h.npmUser = p.Email
	h.npmPass = p.Password
	h.mu.Unlock()
}

func (h *ProxyHandler) persist(url, email, password string) {
	if h.store == nil {
		return
	}
	b, err := json.Marshal(npmPersisted{URL: url, Email: email, Password: password})
	if err == nil {
		_ = h.store.SetSetting(context.Background(), npmSettingKey, string(b))
	}
}

type npmConfig struct {
	URL      string `json:"url"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Configure sets the NPM connection details.
func (h *ProxyHandler) Configure(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	var cfg npmConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if cfg.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	token, exp, err := h.authenticate(dialURL(cfg.URL), cfg.Email, cfg.Password)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to authenticate with NPM: "+err.Error())
		return
	}

	h.mu.Lock()
	h.npmURL = cfg.URL
	h.npmToken = token
	h.npmExp = exp
	h.npmUser = cfg.Email
	h.npmPass = cfg.Password
	h.mu.Unlock()

	// Persist so the connection survives a controller restart/redeploy.
	h.persist(cfg.URL, cfg.Email, cfg.Password)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connected": true,
		"url":       cfg.URL,
		"expires":   exp.Format(time.RFC3339),
	})
}

// Disconnect clears the stored NPM connection (URL, credentials, and token) so
// the dashboard forgets it and shows the setup form again. NPM itself and any
// proxy hosts it manages are left untouched — this only drops Stack Manager's
// saved link to it.
func (h *ProxyHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	h.mu.Lock()
	h.npmURL = ""
	h.npmToken = ""
	h.npmUser = ""
	h.npmPass = ""
	h.npmExp = time.Time{}
	h.mu.Unlock()

	if h.store != nil {
		_ = h.store.SetSetting(context.Background(), npmSettingKey, "")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"connected": false})
}

// npmContainerID finds the running Nginx Proxy Manager container by image name.
func npmContainerID() string {
	res, err := core.DockerExec("ps", "--format", "{{.ID}}\t{{.Image}}")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && strings.Contains(parts[1], "nginx-proxy-manager") {
			return strings.TrimSpace(parts[0])
		}
	}
	return ""
}

// npmGateway returns the Docker network gateway of the NPM container. That
// address reaches host-published ports from *inside* the NPM container without
// the public-IP hairpin that causes 504s, so it's the correct forward host when
// proxying a service that runs on the same host as NPM.
func npmGateway() string {
	id := npmContainerID()
	if id == "" {
		return ""
	}
	res, err := core.DockerExec("inspect", "--format", "{{range .NetworkSettings.Networks}}{{.Gateway}} {{end}}", id)
	if err != nil {
		return ""
	}
	for _, gw := range strings.Fields(res.Stdout) {
		if gw != "" {
			return gw
		}
	}
	return ""
}

// ForwardHost returns the recommended forward-host address for proxying a
// same-host service through NPM (NPM's Docker gateway). The dashboard uses it to
// prefill "Add to Proxy" targets instead of the host's public IP.
func (h *ProxyHandler) ForwardHost(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"forward_host": npmGateway()})
}

// Status returns whether NPM is configured and reachable.
func (h *ProxyHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	h.mu.RLock()
	url := h.npmURL
	hasToken := h.npmToken != ""
	exp := h.npmExp
	h.mu.RUnlock()

	connected := hasToken && time.Now().Before(exp)
	// If we have a persisted URL + credentials but no live token (e.g. right
	// after a restart), re-authenticate so the tab reconnects automatically.
	if url != "" && !connected {
		if _, _, err := h.ensureToken(); err == nil {
			connected = true
			h.mu.RLock()
			exp = h.npmExp
			h.mu.RUnlock()
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configured": url != "",
		"connected":  connected,
		"url":        url,
		"expires":    exp.Format(time.RFC3339),
	})
}

func generateNPMPassword() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// NPM requires >= 8 chars; hex is safe. The "Sm" prefix guarantees a letter
	// start in case NPM ever enforces a leading non-digit.
	return "Sm" + hex.EncodeToString(b), nil
}

func (h *ProxyHandler) npmPutRaw(dial, token, path string, body []byte) error {
	req, _ := http.NewRequest("PUT", dial+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("PUT %s -> %d: %s", path, resp.StatusCode, truncate(string(raw), 200))
	}
	return nil
}

// initNPMDefaults waits for a freshly deployed NPM to be reachable, logs in with
// the first-run default admin@example.com/changeme, and changes the password to
// a generated one (NPM forces changing the default). Returns the admin email +
// new password. Fails if NPM was already initialized (default login rejected).
func (h *ProxyHandler) initNPMDefaults(dial string) (string, string, error) {
	const defEmail = "admin@example.com"
	const defPass = "changeme"
	var token string
	var err error
	deadline := time.Now().Add(45 * time.Second)
	for {
		token, _, err = h.authenticate(dial, defEmail, defPass)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return "", "", fmt.Errorf("NPM not ready or already initialized: %w", err)
		}
		time.Sleep(2 * time.Second)
	}
	newPass, err := generateNPMPassword()
	if err != nil {
		return "", "", err
	}
	// Best-effort profile fill (NPM's first run wants a real name).
	profile, _ := json.Marshal(map[string]interface{}{"name": "Administrator", "nickname": "Admin", "email": defEmail})
	_ = h.npmPutRaw(dial, token, "/api/users/1", profile)
	authBody, _ := json.Marshal(map[string]string{"type": "password", "current": defPass, "secret": newPass})
	if err := h.npmPutRaw(dial, token, "/api/users/1/auth", authBody); err != nil {
		return "", "", fmt.Errorf("failed to set NPM password: %w", err)
	}
	return defEmail, newPass, nil
}

// DeployNPM creates and starts a Nginx Proxy Manager project from the built-in
// template, so the operator can stand NPM up with one click before connecting.
// If the project already exists it is just brought up. NPM's first-run default
// login is admin@example.com / changeme — the caller connects with those, then
// NPM forces a password change on first UI login.
func (h *ProxyHandler) DeployNPM(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	const projectName = "nginx-proxy-manager"

	var tmpl *core.StackTemplate
	for i := range core.BuiltinStackTemplates() {
		t := core.BuiltinStackTemplates()[i]
		if t.ID == projectName {
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		writeError(w, http.StatusInternalServerError, "nginx-proxy-manager template not found")
		return
	}

	// The NPM template references the shared stackmgr-net network as external, so
	// it must exist before compose up (idempotent; normally already created for
	// the Stack Manager stack by prepare-state.sh).
	_, _ = core.DockerExec("network", "create", "stackmgr-net")

	project, err := h.engine.GetProject(projectName)
	if err != nil {
		// Not present yet — create it from the template.
		project, err = h.engine.CreateProject(core.CreateProjectRequest{
			Name:           projectName,
			ComposeContent: tmpl.ComposeContent,
			EnvContent:     tmpl.EnvContent,
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to create NPM project: "+err.Error())
			return
		}
	}

	result := h.engine.Up(project)
	if result != nil && !result.Success {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"status":  "error",
			"project": projectName,
			"error":   "docker compose up failed",
			"output":  result.Output,
		})
		return
	}

	// Auto-initialize NPM's admin (set a real password) and wire the connection
	// so it "just works" without the operator touching NPM's UI. Prefer the
	// shared-network alias (admin :81 is bound to localhost on the host and only
	// reachable over stackmgr-net); fall back to the host gateway for older NPM
	// deployments that still publish :81 on the host.
	npmURL := "http://stackmgr-npm:81"
	email, pass, initErr := h.initNPMDefaults(npmURL)
	if initErr != nil {
		const fallbackURL = "http://localhost:81"
		var fbErr error
		email, pass, fbErr = h.initNPMDefaults(dialURL(fallbackURL))
		if fbErr != nil {
			// NPM is up but auto-setup didn't run (already initialized, or slow
			// to start). Fall back to manual connect.
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"deployed":         true,
				"project":          projectName,
				"auto_configured":  false,
				"default_login":    "admin@example.com",
				"default_password": "changeme",
				"note":             "NPM deployed. Auto-setup skipped (" + fbErr.Error() + "). Connect in Settings > Reverse Proxy.",
			})
			return
		}
		npmURL = fallbackURL
	}
	h.mu.Lock()
	h.npmURL = npmURL
	h.npmToken = ""
	h.npmUser = email
	h.npmPass = pass
	h.mu.Unlock()
	h.persist(npmURL, email, pass)
	_, _, _ = h.ensureToken() // prime a token so the tab shows connected immediately
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deployed":        true,
		"project":         projectName,
		"auto_configured": true,
		"url":             npmURL,
		"login":           email,
		"password":        pass,
		"note":            "NPM deployed and auto-connected over the shared network. Save this password — it's also the NPM admin UI login (admin@example.com). Change it in NPM when convenient. The admin port :81 is bound to localhost only (not internet-exposed).",
	})
}

// ListHosts returns proxy hosts from NPM.
func (h *ProxyHandler) ListHosts(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	body, err := h.npmGet("/api/nginx/proxy-hosts")
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
}

// CreateHost creates a new proxy host in NPM.
func (h *ProxyHandler) CreateHost(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.npmPost("/api/nginx/proxy-hosts", body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(resp) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
}

// DeleteHost removes a proxy host from NPM.
func (h *ProxyHandler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	_, err := h.npmDelete("/api/nginx/proxy-hosts/" + id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// ProjectSuggestions returns discovered projects with their exposed
// ports so the UI can pre-fill the proxy host creation form.
func (h *ProxyHandler) ProjectSuggestions(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	projects, err := h.engine.DiscoverProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type suggestion struct {
		Name  string `json:"name"`
		Ports string `json:"ports"`
	}
	var suggestions []suggestion
	for _, p := range projects {
		if !p.Running {
			continue
		}
		for _, c := range p.Containers {
			if c.Ports != "" {
				suggestions = append(suggestions, suggestion{
					Name:  p.Name,
					Ports: c.Ports,
				})
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, suggestions)
}

// --- NPM HTTP helpers --------------------------------------------------------

// dialURL rewrites a localhost/loopback NPM URL to host.docker.internal so the
// containerized server can actually reach NPM published on the host. This lets
// the operator connect the reverse proxy "over localhost" without exposing NPM's
// admin port externally. Requires the server service to have
// extra_hosts: host.docker.internal:host-gateway (set in docker-compose.yml).
func dialURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		host := "host.docker.internal"
		if p := u.Port(); p != "" {
			host += ":" + p
		}
		u.Host = host
		return u.String()
	}
	return raw
}

func (h *ProxyHandler) ensureToken() (string, string, error) {
	h.mu.RLock()
	rawURL := h.npmURL
	token := h.npmToken
	exp := h.npmExp
	user := h.npmUser
	pass := h.npmPass
	h.mu.RUnlock()

	if rawURL == "" {
		return "", "", fmt.Errorf("NPM not configured — set the connection in Settings > Reverse Proxy")
	}
	// Connect over the host gateway when configured as localhost so the request
	// leaves the container and reaches NPM's published port on the host.
	dial := dialURL(rawURL)
	if token != "" && time.Now().Before(exp.Add(-1*time.Minute)) {
		return dial, token, nil
	}
	newToken, newExp, err := h.authenticate(dial, user, pass)
	if err != nil {
		return "", "", err
	}
	h.mu.Lock()
	h.npmToken = newToken
	h.npmExp = newExp
	h.mu.Unlock()
	return dial, newToken, nil
}

func (h *ProxyHandler) authenticate(baseURL, email, password string) (string, time.Time, error) {
	// NPM's POST /api/tokens schema is additionalProperties:false and accepts
	// only identity + secret. Sending anything else (e.g. an "expiry" field)
	// fails with HTTP 400 "data must NOT have additional properties". NPM issues
	// the token with a default expiry which we read from the response and
	// refresh before it lapses (see ensureToken).
	body, _ := json.Marshal(map[string]string{
		"identity": email,
		"secret":   password,
	})
	resp, err := http.Post(baseURL+"/api/tokens", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("auth failed (HTTP %d): %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var result struct {
		Token   string `json:"token"`
		Expires string `json:"expires"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("invalid auth response: %w", err)
	}
	exp, _ := time.Parse(time.RFC3339, result.Expires)
	if exp.IsZero() {
		exp = time.Now().Add(365 * 24 * time.Hour)
	}
	return result.Token, exp, nil
}

func (h *ProxyHandler) npmGet(path string) ([]byte, error) {
	url, token, err := h.ensureToken()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", url+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("NPM API %s returned %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func (h *ProxyHandler) npmPost(path string, payload json.RawMessage) ([]byte, error) {
	url, token, err := h.ensureToken()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("POST", url+path, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("NPM API %s returned %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func (h *ProxyHandler) npmDelete(path string) ([]byte, error) {
	url, token, err := h.ensureToken()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("DELETE", url+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("NPM API %s returned %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}
