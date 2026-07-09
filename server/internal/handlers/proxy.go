package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
)

// ProxyHandler manages communication with a Nginx Proxy Manager
// instance. It stores the NPM base URL and credentials in memory
// (set from the Settings UI) and proxies API calls so the dashboard
// can list, create, and delete proxy hosts without the user touching
// NPM's admin panel directly.
type ProxyHandler struct {
	engine *core.Engine

	mu       sync.RWMutex
	npmURL   string
	npmToken string
	npmExp   time.Time
	npmUser  string
	npmPass  string
}

func NewProxyHandler(engine *core.Engine) *ProxyHandler {
	return &ProxyHandler{engine: engine}
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

	token, exp, err := h.authenticate(cfg.URL, cfg.Email, cfg.Password)
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connected": true,
		"url":       cfg.URL,
		"expires":   exp.Format(time.RFC3339),
	})
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configured": url != "",
		"connected":  hasToken && time.Now().Before(exp),
		"url":        url,
		"expires":    exp.Format(time.RFC3339),
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

func (h *ProxyHandler) ensureToken() (string, string, error) {
	h.mu.RLock()
	url := h.npmURL
	token := h.npmToken
	exp := h.npmExp
	user := h.npmUser
	pass := h.npmPass
	h.mu.RUnlock()

	if url == "" {
		return "", "", fmt.Errorf("NPM not configured — set the connection in Settings > Reverse Proxy")
	}
	if token != "" && time.Now().Before(exp.Add(-1*time.Minute)) {
		return url, token, nil
	}
	newToken, newExp, err := h.authenticate(url, user, pass)
	if err != nil {
		return "", "", err
	}
	h.mu.Lock()
	h.npmToken = newToken
	h.npmExp = newExp
	h.mu.Unlock()
	return url, newToken, nil
}

func (h *ProxyHandler) authenticate(baseURL, email, password string) (string, time.Time, error) {
	body, _ := json.Marshal(map[string]string{
		"identity": email,
		"secret":   password,
		"expiry":   "1y",
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
