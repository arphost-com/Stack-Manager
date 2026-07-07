package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
)

type AgentCheckinHandler struct {
	Store *storage.Store
}

func NewAgentCheckinHandler(store *storage.Store) *AgentCheckinHandler {
	return &AgentCheckinHandler{Store: store}
}

func (h *AgentCheckinHandler) Projects(w http.ResponseWriter, r *http.Request) {
	var req core.AgentProjectCheckin
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.Header.Get("X-Agent-Token")
	}
	agent, err := h.Store.GetAgentByName(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unknown agent")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(agent.Token)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !agent.Enabled {
		writeError(w, http.StatusForbidden, "agent is disabled")
		return
	}
	if err := h.Store.SaveAgentProjectSnapshot(r.Context(), agent.ID, req.Projects); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.Store.TouchAgent(r.Context(), agent.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id": agent.ID,
		"projects": len(req.Projects),
	})
}
