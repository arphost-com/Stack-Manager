package core

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// composeFileNames is the priority order for compose file detection.
var composeFileNames = []string{
	"compose.yml",
	"compose.yaml",
	"docker-compose.yml",
	"docker-compose.yaml",
}

// composeFileForDir returns the path to the compose file in dir, or empty string.
func composeFileForDir(dir string) string {
	for _, name := range composeFileNames {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// sanitizeProjectName converts a folder name to a valid Docker Compose project name.
func sanitizeProjectName(name string) string {
	s := strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.TrimLeft(s, "-")
	s = strings.TrimRight(s, "-")
	// Must start with alphanumeric
	reLeading := regexp.MustCompile(`^[^a-z0-9]+`)
	s = reLeading.ReplaceAllString(s, "")
	if s == "" {
		s = "p"
	}
	return s
}

// DiscoverProjects finds all compose projects under the primary root
// and any configured extra roots.
func (e *Engine) DiscoverProjects() ([]Project, error) {
	projects := make([]Project, 0)
	seen := map[string]bool{}

	for _, root := range e.AllRoots() {
		found, err := e.discoverInRoot(root)
		if err != nil {
			continue
		}
		for _, p := range found {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			projects = append(projects, p)
		}
	}

	if len(projects) == 0 {
		if _, err := os.Stat(e.RootDir); os.IsNotExist(err) {
			return nil, &ErrNotFound{Msg: "root directory does not exist: " + e.RootDir}
		}
	}

	return projects, nil
}

func (e *Engine) discoverInRoot(rootDir string) ([]Project, error) {
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		return nil, err
	}

	projects := make([]Project, 0)

	if cf := composeFileForDir(rootDir); cf != "" {
		p := e.buildProject(rootDir, cf)
		projects = append(projects, p)
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(rootDir, entry.Name())
		cf := composeFileForDir(dir)
		if cf == "" {
			continue
		}
		p := e.buildProject(dir, cf)
		projects = append(projects, p)
	}

	return projects, nil
}

// buildProject creates a Project struct from a directory.
func (e *Engine) buildProject(dir, composeFile string) Project {
	name := filepath.Base(dir)
	p := Project{
		Name:        name,
		Dir:         dir,
		ComposeFile: composeFile,
		Inactive:    e.isInactive(dir),
		HasHook:     make(map[string]bool),
	}

	// Check for hooks
	for _, cmd := range []string{"update", "pull", "restart", "down", "up"} {
		hookPath := e.hookPath("post", cmd, name)
		if info, err := os.Stat(hookPath); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			p.HasHook[cmd] = true
		}
	}

	// Check the actual Docker state and include non-running containers so
	// restart loops and stopped services remain visible to operators.
	containers, state := e.getContainers(name)
	p.State = state
	p.Running = state == "running"
	p.Containers = containers
	p.ImageSources = e.ImageSources(&p)
	p.Documentation = e.ProjectDocs(&p)

	// Detect a .git repo so the UI can show a git pull action.
	if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
		p.IsGit = true
	}

	return p
}

// isInactive checks if a project has the inactive marker file.
func (e *Engine) isInactive(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, inactiveMarker))
	return err == nil
}

// FilterProjects applies filters to a list of projects.
func FilterProjects(projects []Project, only, exclude []string, includeInactive, onlyInactive, runningOnly bool) []Project {
	out := make([]Project, 0, len(projects))
	onlySet := toSet(only)
	excludeSet := toSet(exclude)

	for _, p := range projects {
		if onlyInactive {
			if !p.Inactive {
				continue
			}
		} else if p.Inactive && !includeInactive {
			continue
		}

		if len(onlySet) > 0 {
			if _, ok := onlySet[p.Name]; !ok {
				continue
			}
		}

		if len(excludeSet) > 0 {
			if _, ok := excludeSet[p.Name]; ok {
				continue
			}
		}

		if runningOnly && !p.Running {
			continue
		}

		out = append(out, p)
	}

	return out
}

func toSet(items []string) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}
