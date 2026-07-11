package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

// PortSyncer is implemented by the firewall skill. It auto-adds
// project ports to CSF's csf.allow after a project is started.
type PortSyncer interface {
	SyncProjectPorts(ctx context.Context, portStrings []string) []string
}

// ProjectHandler handles all project-related API endpoints.
type ProjectHandler struct {
	Engine       *core.Engine
	Jobs         *core.JobManager
	Store        *storage.Store
	UpdateChecks *core.UpdateCheckManager
	PortSyncer   PortSyncer
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(engine *core.Engine, jobs *core.JobManager, store *storage.Store) *ProjectHandler {
	return &ProjectHandler{Engine: engine, Jobs: jobs, Store: store}
}

func (h *ProjectHandler) SetUpdateCheckManager(manager *core.UpdateCheckManager) {
	h.UpdateChecks = manager
}

// syncPorts fires a best-effort CSF port sync after a project starts.
// Runs in a goroutine so it doesn't slow down the API response.
func (h *ProjectHandler) syncPorts(ctx context.Context, projectName string) {
	if h.PortSyncer == nil {
		return
	}
	go func() {
		project, err := h.Engine.GetProject(projectName)
		if err != nil || project == nil {
			return
		}
		var portStrings []string
		for _, c := range project.Containers {
			if c.Ports != "" {
				portStrings = append(portStrings, c.Ports)
			}
		}
		h.PortSyncer.SyncProjectPorts(ctx, portStrings)
	}()
}

// Create creates a new compose project under the configured root.
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req core.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	project, err := h.Engine.CreateProject(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.Store.DeleteCache(r.Context(), "projects:list")
	h.applyPolicy(project)
	writeJSON(w, http.StatusCreated, project)
}

// Delete removes an inactive compose project directory.
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req core.DeleteProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.Engine.DeleteProject(name, req)
	// Always invalidate the projects cache so a subsequent list sees whatever
	// state the filesystem is actually in. This matters when compose down
	// succeeded but RemoveAll partially removed files, or when the operator
	// deleted the directory out-of-band - the stale cache would keep the
	// entry visible and mask the actual state.
	h.Store.DeleteCache(r.Context(), "projects:list")
	if err != nil {
		if _, ok := err.(*core.ErrNotFound); ok {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if result != nil && !result.Success {
			writeErrorWithData(w, http.StatusBadRequest, err.Error(), result)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// List returns all discovered projects.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")

	var projects []core.Project

	// Include local projects unless filtering to a specific agent.
	if source == "" || source == "all" || source == "local" {
		local, err := h.discoverProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range local {
			local[i].SourceHost = "local"
		}
		projects = append(projects, local...)
	}

	// Include agent projects when viewing all or a specific agent.
	if source == "" || source == "all" || (source != "local" && source != "") {
		if h.Store != nil {
			agents, _ := h.Store.ListAgents(r.Context())
			for _, agent := range agents {
				if !agent.Enabled {
					continue
				}
				if source != "" && source != "all" && source != agent.Name {
					continue
				}
				// Peer controllers are live-fetched from their /api/v1 rather
				// than read from a check-in snapshot: a peer is a full
				// controller that never pushes to us.
				if agent.Mode == "peer" {
					peerProjects, err := fetchPeerProjects(r.Context(), &agent, r.URL.RawQuery)
					if err != nil {
						// Skip an unreachable peer instead of failing the whole
						// dashboard; its containers just won't appear this load.
						continue
					}
					// Peers don't push, so stamp last_seen on a successful live
					// fetch — otherwise a healthy peer looks like it never checks in.
					_ = h.Store.TouchAgent(r.Context(), agent.ID)
					projects = append(projects, peerProjects...)
					continue
				}
				snapshot, err := h.Store.GetAgentProjectSnapshot(r.Context(), agent.ID)
				if err != nil || snapshot == nil {
					continue
				}
				for i := range snapshot.Projects {
					snapshot.Projects[i].SourceHost = agent.Name
				}
				projects = append(projects, snapshot.Projects...)
			}
		}
	}

	// Apply query filters
	only := r.URL.Query()["only"]
	exclude := r.URL.Query()["exclude"]
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	onlyInactive := r.URL.Query().Get("only_inactive") == "true"
	runningOnly := r.URL.Query().Get("running_only") == "true"

	filtered := core.FilterProjects(projects, only, exclude, includeInactive, onlyInactive, runningOnly)
	writeJSON(w, http.StatusOK, filtered)
}

// Images returns image source metadata and registry accessibility for a project.
func (h *ProjectHandler) Images(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": project.Name,
		"images":  h.checkImageSources(r.Context(), project),
	})
}

// Docs returns project-local documentation files.
// dockerObjectNameRe matches Docker volume/network names. Rejects anything that
// could be a path/flag/injection before the name is passed to docker.
var dockerObjectNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// Volumes lists the Docker volumes belonging to this project.
func (h *ProjectHandler) Volumes(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	vols, err := h.Engine.ListProjectVolumes(project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"project": project.Name, "volumes": vols})
}

// DeleteVolume removes a single volume belonging to this project. The engine
// re-checks project membership, so a volume outside the stack can't be deleted.
func (h *ProjectHandler) DeleteVolume(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	vol := chi.URLParam(r, "volume")
	if !dockerObjectNameRe.MatchString(vol) {
		writeError(w, http.StatusBadRequest, "invalid volume name")
		return
	}
	if err := h.Engine.RemoveProjectVolume(project, vol); err != nil {
		// Most failures are "volume is in use" — surface as a 409 conflict.
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": vol})
}

// Networks lists the Docker networks belonging to this project.
func (h *ProjectHandler) Networks(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	nets, err := h.Engine.ListProjectNetworks(project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"project": project.Name, "networks": nets})
}

// DeleteNetwork removes a single network belonging to this project. The engine
// re-checks project membership, so a network outside the stack can't be deleted.
func (h *ProjectHandler) DeleteNetwork(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	net := chi.URLParam(r, "network")
	if !dockerObjectNameRe.MatchString(net) {
		writeError(w, http.StatusBadRequest, "invalid network name")
		return
	}
	if err := h.Engine.RemoveProjectNetwork(project, net); err != nil {
		// Most failures are "network has active endpoints" — surface as 409.
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": net})
}

func (h *ProjectHandler) Docs(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, h.Engine.ProjectDocs(project))
}

// DocContent returns one project-local documentation file.
func (h *ProjectHandler) DocContent(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	docPath := r.URL.Query().Get("path")
	content, err := h.Engine.ReadProjectDoc(project, docPath)
	if err != nil {
		if _, ok := err.(*core.ErrNotFound); ok {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, content)
}

// Get returns a single project by name.
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := h.Engine.GetProject(name)
	if err != nil {
		if _, ok := err.(*core.ErrNotFound); ok {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.applyPolicy(project)
	writeJSON(w, http.StatusOK, project)
}

// Status returns docker compose ps for a project.
func (h *ProjectHandler) Status(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	result := h.Engine.Status(project)
	writeJSON(w, http.StatusOK, result)
}

// Pull pulls images for a project.
func (h *ProjectHandler) Pull(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	timeout := h.getTimeout(r)
	result := h.Engine.Pull(project, timeout)
	writeJSON(w, http.StatusOK, result)
}

// Up brings up containers.
func (h *ProjectHandler) Up(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	result := h.Engine.Up(project)
	h.syncPorts(r.Context(), project.Name)
	writeJSON(w, http.StatusOK, result)
}

// Down stops and removes containers.
func (h *ProjectHandler) Down(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	result := h.Engine.Down(project)
	writeJSON(w, http.StatusOK, result)
}

// Update performs a full update (hook or pull+up).
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	timeout := h.getTimeout(r)
	if h.noUpdates(project) {
		writeJSON(w, http.StatusOK, skippedUpdateResults(project))
		return
	}
	results := h.Engine.Update(project, timeout)
	h.syncPorts(r.Context(), project.Name)
	writeJSON(w, http.StatusOK, results)
}

// Restart restarts containers.
func (h *ProjectHandler) Restart(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	result := h.Engine.Restart(project)
	writeJSON(w, http.StatusOK, result)
}

// StartJob starts a tracked compose action and returns a job ID immediately.
func (h *ProjectHandler) StartJob(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	action := chi.URLParam(r, "action")
	timeout := h.getTimeout(r)
	if action == "update" && h.noUpdates(project) {
		policy := h.Store.ResolveUpdatePolicy(*project)
		output := "updates skipped: " + policy.NoUpdatesReason + "\n"
		job, err := h.Jobs.StartSkipped(project, action, output)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, job)
		return
	}
	job, err := h.Jobs.Start(h.Engine, project, action, timeout)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// GetUpdatePolicy returns the update policy for a project.
func (h *ProjectHandler) GetUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	policy, err := h.Store.GetProjectPolicy(r.Context(), *project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// SetUpdatePolicy updates the manual update policy for a project.
func (h *ProjectHandler) SetUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	project, err := h.getProject(w, r)
	if err != nil {
		return
	}
	var body struct {
		Mode  string `json:"mode"`
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	policy, err := h.Store.SetProjectPolicy(r.Context(), *project, body.Mode, body.Notes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// CheckUpdates refreshes cached image update availability for active projects.
func (h *ProjectHandler) CheckUpdates(w http.ResponseWriter, r *http.Request) {
	if h.UpdateChecks == nil {
		writeError(w, http.StatusServiceUnavailable, "update checker is not configured")
		return
	}
	status := h.UpdateChecks.Run(r.Context())
	h.Store.DeleteCache(r.Context(), "projects:list")
	writeJSON(w, http.StatusOK, status)
}

// GetJob returns a tracked compose action with its current or completed output.
func (h *ProjectHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobId")
	job, ok := h.Jobs.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// ListJobs returns tracked compose action sessions.
func (h *ProjectHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Jobs.List())
}

// SetInactive toggles the inactive marker.
func (h *ProjectHandler) SetInactive(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Inactive bool `json:"inactive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.Engine.SetInactive(name, body.Inactive); err != nil {
		if _, ok := err.(*core.ErrNotFound); ok {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.Store.DeleteCache(r.Context(), "projects:list")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":  name,
		"inactive": body.Inactive,
	})
}

// BulkAction performs an action on multiple projects.
func (h *ProjectHandler) BulkAction(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")

	var req core.BulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	projects, err := h.discoverProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filtered := core.FilterProjects(projects, req.Projects, req.Exclude, false, false, false)

	var results []core.OpResult
	for _, p := range filtered {
		p := p
		var result *core.OpResult
		switch action {
		case "pull":
			result = h.Engine.Pull(&p, req.Timeout)
		case "up":
			result = h.Engine.Up(&p)
		case "down":
			result = h.Engine.Down(&p)
		case "restart":
			result = h.Engine.Restart(&p)
		case "update":
			if h.noUpdates(&p) {
				results = append(results, skippedUpdateResults(&p)...)
				continue
			}
			subResults := h.Engine.Update(&p, req.Timeout)
			results = append(results, subResults...)
			continue
		default:
			writeError(w, http.StatusBadRequest, "invalid action: "+action)
			return
		}
		results = append(results, *result)
	}

	successes := 0
	failures := 0
	for _, r := range results {
		if r.Success {
			successes++
		} else {
			failures++
		}
	}

	writeJSON(w, http.StatusOK, core.BulkResult{
		Results: results,
		Total:   len(results),
		Success: successes,
		Failed:  failures,
	})
}

// Prune runs a selected Docker prune command.
func (h *ProjectHandler) Prune(w http.ResponseWriter, r *http.Request) {
	var req core.PruneRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Mode == "" {
		req.Mode = r.URL.Query().Get("mode")
	}
	result := h.Engine.Prune(req.Mode)
	writeJSON(w, http.StatusOK, result)
}

// RegistryLogin logs Docker into a private registry for future pulls.
func (h *ProjectHandler) RegistryLogin(w http.ResponseWriter, r *http.Request) {
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

// ListRegistryLogins returns saved docker logins with passwords redacted.
func (h *ProjectHandler) ListRegistryLogins(w http.ResponseWriter, r *http.Request) {
	logins, err := core.ListSavedRegistryLogins()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logins)
}

// DeleteRegistryLogin removes a saved docker login by registry name.
func (h *ProjectHandler) DeleteRegistryLogin(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "registry")
	// chi refuses to decode %2F (encoded slash) inside URL parameters as a
	// security measure - encoded slashes in a path can confuse routing. The
	// Docker Hub registry key is "https://index.docker.io/v1/" which contains
	// slashes, so the client encodes them and we must decode them here
	// ourselves. Without this step DeleteSavedRegistryLogin was called with
	// the raw "https%3A%2F%2Findex.docker.io%2Fv1%2F" string and reported
	// success against a registry entry that never existed while the real
	// entry was left in config.json.
	registry, err := url.PathUnescape(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "registry parameter is not valid URL encoding")
		return
	}
	result := core.DeleteSavedRegistryLogin(registry)
	if !result.Success {
		writeErrorWithData(w, http.StatusBadRequest, "docker logout failed", result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// getProject is a helper that extracts and validates the project name from URL.
func (h *ProjectHandler) getProject(w http.ResponseWriter, r *http.Request) (*core.Project, error) {
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
	h.applyPolicy(project)
	return project, nil
}

// getTimeout reads timeout from query string.
func (h *ProjectHandler) getTimeout(r *http.Request) int {
	if t := r.URL.Query().Get("timeout"); t != "" {
		if v, err := strconv.Atoi(t); err == nil {
			return v
		}
	}
	return 0
}

func (h *ProjectHandler) discoverProjects(ctx context.Context) ([]core.Project, error) {
	var cached []core.Project
	if h.Store.GetJSON(ctx, "projects:list", &cached) {
		return cached, nil
	}
	projects, err := h.Engine.DiscoverProjects()
	if err != nil {
		return nil, err
	}
	for i := range projects {
		h.applyPolicy(&projects[i])
	}
	h.Store.SetJSON(ctx, "projects:list", projects, h.Store.CacheTTL)
	return projects, nil
}

func (h *ProjectHandler) applyPolicy(project *core.Project) {
	if project == nil {
		return
	}
	project.UpdatePolicy = h.Store.ResolveUpdatePolicy(*project)
	if status, err := h.Store.ProjectUpdateStatus(context.Background(), project.Name); err == nil {
		project.UpdateStatus = status
	}
}

func (h *ProjectHandler) noUpdates(project *core.Project) bool {
	if project == nil {
		return false
	}
	policy := h.Store.ResolveUpdatePolicy(*project)
	project.UpdatePolicy = policy
	return policy.EffectivePolicy == core.UpdatePolicyNoUpdates
}

func skippedUpdateResults(project *core.Project) []core.OpResult {
	reason := "updates disabled"
	if project.UpdatePolicy.NoUpdatesReason != "" {
		reason = project.UpdatePolicy.NoUpdatesReason
	}
	return []core.OpResult{{
		Project:  project.Name,
		Action:   "update",
		Success:  true,
		Output:   "updates skipped: " + reason + "\n",
		ExitCode: 0,
	}}
}

func (h *ProjectHandler) checkImageSources(ctx context.Context, project *core.Project) []core.ImageSource {
	key := "project_images:" + project.Name
	var cached []core.ImageSource
	if h.Store.GetJSON(ctx, key, &cached) {
		return cached
	}
	images := h.Engine.CheckImageSources(project)
	h.Store.SetJSON(ctx, key, images, h.Store.CacheTTL)
	return images
}
