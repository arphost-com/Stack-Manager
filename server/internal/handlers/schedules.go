package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type ScheduleHandler struct {
	Store     *storage.Store
	Scheduler *core.ScheduleManager
}

func NewScheduleHandler(store *storage.Store, scheduler *core.ScheduleManager) *ScheduleHandler {
	return &ScheduleHandler{Store: store, Scheduler: scheduler}
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.Store.ListSchedules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (h *ScheduleHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req core.UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	normalizeSchedule(&req)
	if err := validateSchedule(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	schedule, err := h.Store.SaveSchedule(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64Param(r, "scheduleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Store.DeleteSchedule(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func (h *ScheduleHandler) RunNow(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64Param(r, "scheduleID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	schedule, err := h.Store.GetSchedule(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.Scheduler == nil {
		writeError(w, http.StatusInternalServerError, "scheduler is not running")
		return
	}
	if err := h.Scheduler.RunNow(r.Context(), *schedule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.Store.GetSchedule(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusAccepted, schedule)
		return
	}
	writeJSON(w, http.StatusAccepted, updated)
}

type AgentHandler struct {
	Store      *storage.Store
	HTTPClient *http.Client
}

func NewAgentHandler(store *storage.Store) *AgentHandler {
	return &AgentHandler{
		Store:      store,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	agents, err := h.Store.ListAgents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *AgentHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req core.ComposeAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode == "" {
		if req.BaseURL == "" {
			req.Mode = "callback"
		} else {
			req.Mode = "inbound"
		}
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}
	if req.Mode != "inbound" && req.Mode != "callback" {
		writeError(w, http.StatusBadRequest, "agent mode must be inbound or callback")
		return
	}
	if req.Mode == "inbound" && req.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "agent URL is required for inbound mode")
		return
	}
	if req.Mode == "callback" {
		req.BaseURL = ""
	}
	if strings.TrimSpace(req.Token) == "" {
		if _, err := h.Store.GetAgentByName(r.Context(), req.Name); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "agent token is required for new agents")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	agent, err := h.Store.SaveAgent(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64Param(r, "agentID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Store.DeleteAgent(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func (h *AgentHandler) Projects(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64Param(r, "agentID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agent, err := h.Store.GetAgent(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !agent.Enabled {
		writeError(w, http.StatusBadRequest, "agent is disabled")
		return
	}
	if agent.BaseURL == "" || agent.Mode == "callback" {
		snapshot, err := h.Store.GetAgentProjectSnapshot(r.Context(), agent.ID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeJSON(w, http.StatusOK, []core.Project{})
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot.Projects)
		return
	}
	base, err := url.Parse(agent.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		writeError(w, http.StatusBadRequest, "invalid agent URL")
		return
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		writeError(w, http.StatusBadRequest, "agent URL must use http or https")
		return
	}
	path := strings.TrimRight(base.String(), "/") + "/agent/v1/projects"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, path, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+agent.Token)
	client := h.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer res.Body.Close()
	var envelope struct {
		Status string         `json:"status"`
		Error  string         `json:"error"`
		Data   []core.Project `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || envelope.Status == "error" {
		if envelope.Error == "" {
			envelope.Error = res.Status
		}
		writeError(w, http.StatusBadGateway, envelope.Error)
		return
	}
	_ = h.Store.TouchAgent(r.Context(), agent.ID)
	writeJSON(w, http.StatusOK, envelope.Data)
}

func ListStackTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, core.BuiltinStackTemplates())
}

func GetStackTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "templateID")
	template, ok := core.GetBuiltinStackTemplate(id)
	if !ok {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func normalizeSchedule(req *core.UpdateScheduleRequest) {
	req.Project = strings.TrimSpace(req.Project)
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.Action == "" {
		req.Action = "update"
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 300
	}
	if req.NextRunAt != nil {
		next := req.NextRunAt.UTC().Truncate(time.Second)
		req.NextRunAt = &next
	}
}

func validateSchedule(req core.UpdateScheduleRequest) error {
	if req.Project == "" {
		return errors.New("project is required")
	}
	if !core.ValidJobAction(req.Action) {
		return errors.New("invalid action")
	}
	if req.IntervalMinutes < 5 {
		return errors.New("interval must be at least 5 minutes")
	}
	if req.TimeoutSeconds < 0 {
		return errors.New("timeout cannot be negative")
	}
	return nil
}

func parseInt64Param(r *http.Request, name string) (int64, error) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
