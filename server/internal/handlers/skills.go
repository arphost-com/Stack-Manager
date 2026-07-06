package handlers

import (
	"net/http"

	"github.com/arphost-com/Stack-Manager/server/internal/skills"
	"github.com/go-chi/chi/v5"
)

// SkillHandler handles skill listing and info endpoints.
type SkillHandler struct {
	Registry *skills.Registry
}

// NewSkillHandler creates a new SkillHandler.
func NewSkillHandler(registry *skills.Registry) *SkillHandler {
	return &SkillHandler{Registry: registry}
}

// List returns all registered skills with health status.
func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	infos := h.Registry.List(r.Context())
	writeJSON(w, http.StatusOK, infos)
}

// Get returns info for a single skill.
func (h *SkillHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "skillName")
	s, ok := h.Registry.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "skill not found: "+name)
		return
	}

	healthy := s.HealthCheck(r.Context()) == nil
	writeJSON(w, http.StatusOK, skills.SkillInfo{
		Name:        s.Name(),
		Description: s.Description(),
		Version:     s.Version(),
		Healthy:     healthy,
	})
}
