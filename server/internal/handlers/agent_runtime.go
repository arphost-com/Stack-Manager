package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/go-chi/chi/v5"
)

type AgentRuntimeHandler struct {
	Engine *core.Engine
	Jobs   *core.JobManager
	Token  string
	Name   string
}

func NewAgentRuntimeHandler(engine *core.Engine, jobs *core.JobManager, token, name string) *AgentRuntimeHandler {
	return &AgentRuntimeHandler{Engine: engine, Jobs: jobs, Token: token, Name: name}
}

func (h *AgentRuntimeHandler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.Header.Get("X-Agent-Token")
		}
		if token == "" || token != h.Token {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AgentRuntimeHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"mode":   "agent",
		"name":   h.Name,
	})
}

func (h *AgentRuntimeHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.Engine.DiscoverProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *AgentRuntimeHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.projectFromRequest(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *AgentRuntimeHandler) Images(w http.ResponseWriter, r *http.Request) {
	project, err := h.projectFromRequest(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": project.Name,
		"images":  h.Engine.CheckImageSources(project),
	})
}

func (h *AgentRuntimeHandler) Status(w http.ResponseWriter, r *http.Request) {
	project, err := h.projectFromRequest(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, h.Engine.Status(project))
}

func (h *AgentRuntimeHandler) StartJob(w http.ResponseWriter, r *http.Request) {
	project, err := h.projectFromRequest(w, r)
	if err != nil {
		return
	}
	action := chi.URLParam(r, "action")
	timeout := timeoutFromRequest(r)
	job, err := h.Jobs.Start(h.Engine, project, action, timeout)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AgentRuntimeHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Jobs.List())
}

func (h *AgentRuntimeHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobId")
	job, ok := h.Jobs.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *AgentRuntimeHandler) Logs(w http.ResponseWriter, r *http.Request) {
	project, err := h.projectFromRequest(w, r)
	if err != nil {
		return
	}
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	container := r.URL.Query().Get("container")
	if container != "" {
		if !agentProjectHasContainer(project, container) {
			writeError(w, http.StatusBadRequest, "container does not belong to project: "+container)
			return
		}
		result, _ := core.DockerExec("logs", "--tail", tail, "--timestamps", container)
		if result == nil {
			writeJSON(w, http.StatusOK, []map[string]string{})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]string{{
			"container": container,
			"output":    result.Stdout + result.Stderr,
		}})
		return
	}
	result := h.Engine.ExecCompose(project, "logs", "--tail", tail, "--timestamps")
	writeJSON(w, http.StatusOK, []map[string]string{{
		"project": project.Name,
		"output":  result.Output,
	}})
}

func (h *AgentRuntimeHandler) Stats(w http.ResponseWriter, r *http.Request) {
	project, err := h.projectFromRequest(w, r)
	if err != nil {
		return
	}
	var containerNames []string
	for _, c := range project.Containers {
		containerNames = append(containerNames, c.Name)
	}
	if len(containerNames) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"project": project.Name,
			"stats":   []interface{}{},
		})
		return
	}
	args := append([]string{"stats", "--no-stream", "--format",
		`{"container":"{{.Name}}","cpu":"{{.CPUPerc}}","memory":"{{.MemUsage}}","mem_percent":"{{.MemPerc}}","net_io":"{{.NetIO}}","block_io":"{{.BlockIO}}","pids":"{{.PIDs}}"}`},
		containerNames...)
	result, _ := core.DockerExec(args...)
	if result == nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	var stats []json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		var parsed json.RawMessage
		if json.Unmarshal([]byte(line), &parsed) == nil {
			stats = append(stats, parsed)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": project.Name,
		"stats":   stats,
	})
}

func (h *AgentRuntimeHandler) RegistryLogin(w http.ResponseWriter, r *http.Request) {
	var req core.RegistryLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result := core.DockerLogin(req)
	if !result.Success {
		writeJSON(w, http.StatusBadRequest, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AgentRuntimeHandler) ListRegistryLogins(w http.ResponseWriter, r *http.Request) {
	logins, err := core.ListSavedRegistryLogins()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logins)
}

func (h *AgentRuntimeHandler) DeleteRegistryLogin(w http.ResponseWriter, r *http.Request) {
	registry := chi.URLParam(r, "registry")
	result := core.DeleteSavedRegistryLogin(registry)
	if !result.Success {
		writeJSON(w, http.StatusBadRequest, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AgentRuntimeHandler) Prune(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Engine.Prune("safe"))
}

func (h *AgentRuntimeHandler) projectFromRequest(w http.ResponseWriter, r *http.Request) (*core.Project, error) {
	name := chi.URLParam(r, "name")
	project, err := h.Engine.GetProject(name)
	if err != nil {
		if _, ok := err.(*core.ErrNotFound); ok {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return nil, err
	}
	return project, nil
}

func timeoutFromRequest(r *http.Request) int {
	if t := r.URL.Query().Get("timeout"); t != "" {
		if v, err := strconv.Atoi(t); err == nil {
			return v
		}
	}
	return 0
}

func agentProjectHasContainer(project *core.Project, name string) bool {
	for _, container := range project.Containers {
		if container.Name == name || container.ID == name {
			return true
		}
	}
	return false
}
