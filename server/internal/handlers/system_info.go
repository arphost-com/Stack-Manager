package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/arphost-com/Stack-Manager/server/internal/version"
)

// SystemInfoHandler exposes a friendly display name for this controller so the
// UI can show a name instead of the bare IP. Resolution order: the stored
// setting (editable in Settings), then the SERVER_DISPLAY_NAME env, then the OS
// hostname.
type SystemInfoHandler struct {
	Store *storage.Store
}

func NewSystemInfoHandler(store *storage.Store) *SystemInfoHandler {
	return &SystemInfoHandler{Store: store}
}

func (h *SystemInfoHandler) serverName(r *http.Request) string {
	if h.Store != nil {
		if v, ok := h.Store.GetSettingString(r.Context(), "server_display_name"); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	if s := strings.TrimSpace(os.Getenv("SERVER_DISPLAY_NAME")); s != "" {
		return s
	}
	// Deliberately no os.Hostname() fallback: inside a container that's the
	// random container ID, which is useless. Returning empty lets the UI fall
	// back to the address-bar host (the real IP/FQDN the operator uses).
	return ""
}

func (h *SystemInfoHandler) hostURL(r *http.Request) string {
	if h.Store != nil {
		if v, ok := h.Store.GetSettingString(r.Context(), "HOST_URL"); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return strings.TrimSpace(os.Getenv("HOST_URL"))
}

func connectionHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err == nil && u.Host != "" {
		return u.Hostname()
	}
	host := rawURL
	if strings.Contains(host, "://") {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(h, "[]")
	}
	return strings.Trim(strings.TrimPrefix(host, "["), "]")
}

func (h *SystemInfoHandler) Get(w http.ResponseWriter, r *http.Request) {
	hostURL := h.hostURL(r)
	res := map[string]string{
		"server_name":     h.serverName(r),
		"host_url":        hostURL,
		"connection_host": connectionHost(hostURL),
	}
	ver := ""
	if h.Store != nil {
		if v, ok := h.Store.GetSettingString(r.Context(), "app_version"); ok {
			ver = strings.TrimSpace(v)
		}
	}
	// Fall back to the build-baked version if the DB was never stamped (e.g.
	// first boot before the startup stamp, or a build without a SHA).
	if ver == "" {
		ver = version.Full()
	}
	res["version"] = ver
	writeJSON(w, http.StatusOK, res)
}

func (h *SystemInfoHandler) Save(w http.ResponseWriter, r *http.Request) {
	// Pointers so only provided fields update: the UI sends server_name; the
	// deploy script sends app_version (the version stamp, DB-backed).
	var req struct {
		ServerName *string `json:"server_name"`
		AppVersion *string `json:"app_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, "settings store unavailable")
		return
	}
	if req.ServerName != nil {
		if err := h.Store.SetSettingString(r.Context(), "server_display_name", strings.TrimSpace(*req.ServerName)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if req.AppVersion != nil {
		if err := h.Store.SetSettingString(r.Context(), "app_version", strings.TrimSpace(*req.AppVersion)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	hostURL := h.hostURL(r)
	res := map[string]string{
		"server_name":     h.serverName(r),
		"host_url":        hostURL,
		"connection_host": connectionHost(hostURL),
	}
	if v, ok := h.Store.GetSettingString(r.Context(), "app_version"); ok {
		res["version"] = v
	}
	writeJSON(w, http.StatusOK, res)
}
