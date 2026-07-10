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

// authAgent authenticates an inbound agent request by matching the agent name
// against its stored token (Bearer or X-Agent-Token). It writes the error
// response itself and returns nil on failure.
func (h *AgentCheckinHandler) authAgent(w http.ResponseWriter, r *http.Request, name string) *core.ComposeAgent {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.Header.Get("X-Agent-Token")
	}
	agent, err := h.Store.GetAgentByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unknown agent")
			return nil
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(agent.Token)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil
	}
	if !agent.Enabled {
		writeError(w, http.StatusForbidden, "agent is disabled")
		return nil
	}
	return agent
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
	agent := h.authAgent(w, r, req.Name)
	if agent == nil {
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
	// Hand back any commands queued for this agent. They're claimed (marked
	// dispatched) here so they're delivered exactly once; the agent runs them
	// and reports results to Results below.
	commands, err := h.Store.ClaimPendingCommands(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if commands == nil {
		commands = []core.AgentCommandDispatch{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id": agent.ID,
		"projects": len(req.Projects),
		"commands": commands,
	})
}

// Results records the outcome of dispatched commands reported by a callback
// agent after it runs them.
func (h *AgentCheckinHandler) Results(w http.ResponseWriter, r *http.Request) {
	var req core.AgentCommandResults
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}
	agent := h.authAgent(w, r, req.Name)
	if agent == nil {
		return
	}
	for _, res := range req.Results {
		if err := h.Store.SaveAgentCommandResult(r.Context(), agent.ID, res.ID, res.Success, res.Output); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"saved": len(req.Results)})
}
