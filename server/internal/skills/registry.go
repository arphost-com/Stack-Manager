package skills

import (
	"context"
	"fmt"
	"sync"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/go-chi/chi/v5"
)

// Skill is the interface that all skills must implement.
type Skill interface {
	// Name returns the unique skill identifier (used in URL paths).
	Name() string

	// Description returns a human-readable description.
	Description() string

	// Version returns the skill version.
	Version() string

	// Init initializes the skill with the core engine and config.
	Init(ctx context.Context, engine *core.Engine, cfg map[string]interface{}) error

	// Shutdown gracefully stops the skill.
	Shutdown(ctx context.Context) error

	// RegisterRoutes mounts the skill's API routes on the given router.
	// Routes are automatically prefixed with /api/v1/skills/{name}/
	RegisterRoutes(r chi.Router)

	// HealthCheck returns nil if the skill is healthy.
	HealthCheck(ctx context.Context) error
}

// SkillInfo is the serializable metadata for a skill.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Healthy     bool   `json:"healthy"`
}

// Registry manages skill registration and lifecycle.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill
	order  []string
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// Register adds a skill to the registry. Must be called before InitAll.
func (r *Registry) Register(s Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := s.Name()
	if _, exists := r.skills[name]; exists {
		return fmt.Errorf("skill already registered: %s", name)
	}
	r.skills[name] = s
	r.order = append(r.order, name)
	return nil
}

// InitAll initializes all registered skills.
func (r *Registry) InitAll(ctx context.Context, engine *core.Engine, cfg map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range r.order {
		s := r.skills[name]
		if err := s.Init(ctx, engine, cfg); err != nil {
			return fmt.Errorf("failed to initialize skill %s: %w", name, err)
		}
	}
	return nil
}

// ShutdownAll gracefully stops all skills in reverse order.
func (r *Registry) ShutdownAll(ctx context.Context) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := len(r.order) - 1; i >= 0; i-- {
		name := r.order[i]
		r.skills[name].Shutdown(ctx)
	}
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List returns info for all registered skills.
func (r *Registry) List(ctx context.Context) []SkillInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []SkillInfo
	for _, name := range r.order {
		s := r.skills[name]
		healthy := s.HealthCheck(ctx) == nil
		infos = append(infos, SkillInfo{
			Name:        s.Name(),
			Description: s.Description(),
			Version:     s.Version(),
			Healthy:     healthy,
		})
	}
	return infos
}

// MountRoutes mounts all skill routes on the given router under /api/v1/skills/{name}/.
func (r *Registry) MountRoutes(router chi.Router) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range r.order {
		s := r.skills[name]
		router.Route("/"+name, func(sr chi.Router) {
			s.RegisterRoutes(sr)
		})
	}
}
