package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

// ProjectFileHandler provides in-browser editing of project config
// files (compose.yml, .env, etc.). Paths are validated to stay inside
// the project directory — no traversal to the host or other projects.
type ProjectFileHandler struct {
	engine *core.Engine
}

func NewProjectFileHandler(engine *core.Engine) *ProjectFileHandler {
	return &ProjectFileHandler{engine: engine}
}

// Editable file extensions / names. We only expose plain-text config
// files, not binaries, images, or database files.
var editableNames = map[string]bool{
	"compose.yml":          true,
	"compose.yaml":         true,
	"docker-compose.yml":   true,
	"docker-compose.yaml":  true,
	"compose.override.yml": true,
	".env":                 true,
	"Dockerfile":           true,
	"Caddyfile":            true,
	"nginx.conf":           true,
	"Makefile":             true,
}

var editableExts = map[string]bool{
	".yml":  true,
	".yaml": true,
	".env":  true,
	".conf": true,
	".cfg":  true,
	".ini":  true,
	".toml": true,
	".json": true,
	".sh":   true,
	".txt":  true,
	".md":   true,
	".xml":  true,
	".properties": true,
}

const maxEditableSize = 1 << 20 // 1 MiB

type projectFile struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	Editable bool      `json:"editable"`
}

// ListFiles returns the top-level files in the project directory that
// are candidates for editing.
func (h *ProjectFileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := h.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	entries, err := os.ReadDir(project.Dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var files []projectFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		editable := isEditable(entry.Name(), info)
		files = append(files, projectFile{
			Name:     entry.Name(),
			Path:     entry.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Editable: editable,
		})
	}
	writeJSON(w, http.StatusOK, files)
}

// ReadFile returns the content of a single project file.
func (h *ProjectFileHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	filePath := r.URL.Query().Get("path")
	project, err := h.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	absPath, err := safePath(project.Dir, filePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if info.IsDir() || info.Size() > maxEditableSize {
		writeError(w, http.StatusBadRequest, "file is a directory or too large to edit")
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    filePath,
		"content": string(content),
		"size":    info.Size(),
	})
}

// WriteFile saves new content to a project file, creating a .bak
// backup of the previous version.
func (h *ProjectFileHandler) WriteFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := h.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(body.Content) > maxEditableSize {
		writeError(w, http.StatusRequestEntityTooLarge, "content too large")
		return
	}

	absPath, err := safePath(project.Dir, body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create backup if the file already exists.
	if _, statErr := os.Stat(absPath); statErr == nil {
		backupPath := absPath + ".bak"
		_ = copyFileSimple(absPath, backupPath)
	}

	if err := os.WriteFile(absPath, []byte(body.Content), 0o640); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":  body.Path,
		"saved": true,
		"hint":  "Run docker compose up -d to apply compose changes, or restart the relevant service.",
	})
}

// safePath validates that a relative path stays inside the project dir.
func safePath(projectDir, relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.Contains(relPath, "..") || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("path must be relative and cannot contain '..'")
	}
	abs := filepath.Join(projectDir, relPath)
	absClean := filepath.Clean(abs)
	if !strings.HasPrefix(absClean, filepath.Clean(projectDir)+string(filepath.Separator)) && absClean != filepath.Clean(projectDir) {
		return "", fmt.Errorf("path escapes project directory")
	}
	return absClean, nil
}

func isEditable(name string, info fs.FileInfo) bool {
	if info.Size() > maxEditableSize {
		return false
	}
	if editableNames[name] {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	return editableExts[ext]
}

func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o640)
}
