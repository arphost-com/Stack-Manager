package frontend

import (
	"context"
	"net/http"
	"os"

	"github.com/arphost-com/Compose-Manager/server/internal/core"
	"github.com/go-chi/chi/v5"
)

// Skill serves the React SPA frontend.
type Skill struct {
	distDir string
}

func New() *Skill { return &Skill{} }

func (s *Skill) Name() string        { return "frontend" }
func (s *Skill) Description() string { return "Web dashboard UI for managing Docker Compose projects" }
func (s *Skill) Version() string     { return "1.0.0" }

func (s *Skill) Init(_ context.Context, _ *core.Engine, cfg map[string]interface{}) error {
	if dir, ok := cfg["dist_dir"].(string); ok && dir != "" {
		s.distDir = dir
	} else {
		s.distDir = "/app/web/dist"
	}
	return nil
}

func (s *Skill) Shutdown(_ context.Context) error { return nil }

func (s *Skill) HealthCheck(_ context.Context) error {
	if os.Getenv("FRONTEND_EXTERNAL") == "true" {
		return nil
	}
	if _, err := os.Stat(s.distDir); err != nil {
		return err
	}
	return nil
}

// RegisterRoutes mounts the SPA file server.
// The frontend gets mounted at /ui/ in main.go rather than under /api/v1/skills/.
func (s *Skill) RegisterRoutes(r chi.Router) {
	fs := http.FileServer(http.Dir(s.distDir))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file; if not found, serve index.html for SPA routing
		path := s.distDir + r.URL.Path
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, s.distDir+"/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})
}
