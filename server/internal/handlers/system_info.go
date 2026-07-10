package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/arphost-com/Stack-Manager/server/internal/storage"
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

func (h *SystemInfoHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"server_name": h.serverName(r)})
}

func (h *SystemInfoHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerName string `json:"server_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, "settings store unavailable")
		return
	}
	if err := h.Store.SetSettingString(r.Context(), "server_display_name", strings.TrimSpace(req.ServerName)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"server_name": h.serverName(r)})
}
