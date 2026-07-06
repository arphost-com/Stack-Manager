package dbadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/go-chi/chi/v5"
)

// Known database images.
var dbImagePrefixes = map[string]string{
	"postgres":  "postgres",
	"mysql":     "mysql",
	"mariadb":   "mariadb",
	"mongo":     "mongo",
	"redis":     "redis",
}

// Skill implements the database administration skill.
type Skill struct {
	engine  *core.Engine
	dumpDir string
}

func New() *Skill { return &Skill{} }

func (s *Skill) Name() string        { return "dbadmin" }
func (s *Skill) Description() string { return "Database discovery, health checks, dumps, and management for containers running PostgreSQL, MySQL, MariaDB, MongoDB, or Redis" }
func (s *Skill) Version() string     { return "1.0.0" }

func (s *Skill) Init(_ context.Context, engine *core.Engine, cfg map[string]interface{}) error {
	s.engine = engine
	if dir, ok := cfg["backup_dir"].(string); ok && dir != "" {
		s.dumpDir = filepath.Join(dir, "db-dumps")
	} else {
		s.dumpDir = filepath.Join(engine.RootDir, ".stack-manager", "backups", "db-dumps")
	}
	return os.MkdirAll(s.dumpDir, 0755)
}

func (s *Skill) Shutdown(_ context.Context) error { return nil }
func (s *Skill) HealthCheck(_ context.Context) error { return nil }

func (s *Skill) RegisterRoutes(r chi.Router) {
	r.Get("/discover", s.Discover)
	r.Get("/discover/{name}", s.DiscoverProject)
	r.Get("/health/{name}", s.HealthCheck_)
	r.Post("/dump/{name}", s.Dump)
	r.Get("/dumps", s.ListDumps)
	r.Get("/dumps/{name}", s.ListProjectDumps)
}

// Discover finds all database containers across all projects.
func (s *Skill) Discover(w http.ResponseWriter, r *http.Request) {
	projects, err := s.engine.DiscoverProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var databases []core.DatabaseInfo
	for _, p := range projects {
		databases = append(databases, detectDatabases(&p)...)
	}

	writeJSON(w, http.StatusOK, databases)
}

// DiscoverProject finds database containers in a specific project.
func (s *Skill) DiscoverProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	databases := detectDatabases(project)
	writeJSON(w, http.StatusOK, databases)
}

// HealthCheck_ checks if database containers are responding.
func (s *Skill) HealthCheck_(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var results []map[string]interface{}
	for _, c := range project.Containers {
		engine := detectEngine(c.Image)
		if engine == "" {
			continue
		}

		healthy := false
		var checkOutput string

		switch engine {
		case "postgres":
			result, _ := core.DockerExec("exec", c.Name, "pg_isready")
			if result != nil {
				healthy = result.ExitCode == 0
				checkOutput = result.Stdout + result.Stderr
			}
		case "mysql", "mariadb":
			result, _ := core.DockerExec("exec", c.Name, "healthcheck.sh", "--connect")
			if result != nil {
				healthy = result.ExitCode == 0
				checkOutput = result.Stdout + result.Stderr
			}
			if !healthy {
				// Fallback: mysqladmin ping
				result, _ = core.DockerExec("exec", c.Name, "mysqladmin", "ping", "-h", "127.0.0.1")
				if result != nil {
					healthy = result.ExitCode == 0
					checkOutput = result.Stdout + result.Stderr
				}
			}
		case "redis":
			result, _ := core.DockerExec("exec", c.Name, "redis-cli", "ping")
			if result != nil {
				healthy = strings.TrimSpace(result.Stdout) == "PONG"
				checkOutput = result.Stdout
			}
		case "mongo":
			result, _ := core.DockerExec("exec", c.Name, "mongosh", "--eval", "db.adminCommand('ping')")
			if result != nil {
				healthy = result.ExitCode == 0
				checkOutput = result.Stdout + result.Stderr
			}
		}

		results = append(results, map[string]interface{}{
			"container": c.Name,
			"engine":    engine,
			"healthy":   healthy,
			"output":    strings.TrimSpace(checkOutput),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": name,
		"checks":  results,
	})
}

// Dump creates a database dump for a project's database containers.
func (s *Skill) Dump(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	project, err := s.engine.GetProject(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	var dumps []map[string]interface{}

	for _, c := range project.Containers {
		engine := detectEngine(c.Image)
		if engine == "" {
			continue
		}

		dumpFile := fmt.Sprintf("%s__%s__%s.sql", name, c.Name, timestamp)
		dumpPath := filepath.Join(s.dumpDir, dumpFile)

		var dumpCmd *exec.Cmd
		switch engine {
		case "postgres":
			dumpCmd = exec.Command("docker", "exec", c.Name,
				"pg_dumpall", "-U", "postgres")
		case "mysql", "mariadb":
			dumpCmd = exec.Command("docker", "exec", c.Name,
				"mysqldump", "--all-databases", "--single-transaction",
				"--quick", "--routines", "--triggers", "-u", "root",
				fmt.Sprintf("-p%s", getEnvFromContainer(c.Name, "MYSQL_ROOT_PASSWORD", "MARIADB_ROOT_PASSWORD")))
		default:
			continue
		}

		outFile, err := os.Create(dumpPath)
		if err != nil {
			dumps = append(dumps, map[string]interface{}{
				"container": c.Name,
				"engine":    engine,
				"success":   false,
				"error":     err.Error(),
			})
			continue
		}

		dumpCmd.Stdout = outFile
		var stderrBuf strings.Builder
		dumpCmd.Stderr = &stderrBuf
		runErr := dumpCmd.Run()
		outFile.Close()

		success := runErr == nil
		info, _ := os.Stat(dumpPath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		errMsg := ""
		if !success {
			errMsg = strings.TrimSpace(stderrBuf.String())
		}

		dumps = append(dumps, map[string]interface{}{
			"container": c.Name,
			"engine":    engine,
			"success":   success,
			"file":      dumpFile,
			"size":      size,
			"error":     errMsg,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": name,
		"dumps":   dumps,
	})
}

// ListDumps returns all database dumps.
func (s *Skill) ListDumps(w http.ResponseWriter, r *http.Request) {
	dumps := s.listDumpFiles("")
	writeJSON(w, http.StatusOK, dumps)
}

// ListProjectDumps returns database dumps for a specific project.
func (s *Skill) ListProjectDumps(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dumps := s.listDumpFiles(name)
	writeJSON(w, http.StatusOK, dumps)
}

func (s *Skill) listDumpFiles(projectFilter string) []map[string]interface{} {
	var dumps []map[string]interface{}

	entries, err := os.ReadDir(s.dumpDir)
	if err != nil {
		return dumps
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		parts := strings.SplitN(entry.Name(), "__", 3)
		if len(parts) < 2 {
			continue
		}

		if projectFilter != "" && parts[0] != projectFilter {
			continue
		}

		info, _ := entry.Info()
		dumps = append(dumps, map[string]interface{}{
			"file":       entry.Name(),
			"project":    parts[0],
			"container":  parts[1],
			"size_bytes": info.Size(),
			"created_at": info.ModTime(),
		})
	}

	return dumps
}

func detectDatabases(project *core.Project) []core.DatabaseInfo {
	var databases []core.DatabaseInfo
	for _, c := range project.Containers {
		engine := detectEngine(c.Image)
		if engine == "" {
			continue
		}
		databases = append(databases, core.DatabaseInfo{
			Container: c.Name,
			Engine:    engine,
		})
	}
	return databases
}

func detectEngine(image string) string {
	lower := strings.ToLower(image)
	for prefix, engine := range dbImagePrefixes {
		if strings.Contains(lower, prefix) {
			return engine
		}
	}
	return ""
}

func getEnvFromContainer(containerName string, envNames ...string) string {
	for _, envName := range envNames {
		result, _ := core.DockerExec("exec", containerName, "printenv", envName)
		if result != nil && result.ExitCode == 0 {
			val := strings.TrimSpace(result.Stdout)
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "error",
		"error":     msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
