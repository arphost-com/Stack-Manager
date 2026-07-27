package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var composeUserEnvKeys = []string{
	"STACK_UID",
	"STACK_GID",
	"PUID",
	"PGID",
	"UID",
	"GID",
	"USER_UID",
	"USER_GID",
}

// composeRuntimeEnvKeys must continue to come from Stack Manager's own
// process. They control how the Docker CLI reaches the host and finds its
// credentials; a managed project's .env is only for Compose interpolation.
var composeRuntimeEnvKeys = map[string]struct{}{
	"DOCKER_CONFIG":  {},
	"DOCKER_CONTEXT": {},
	"DOCKER_HOST":    {},
	"HOME":           {},
	"PATH":           {},
}

// projectComposeEnv prevents Stack Manager's container environment from
// overriding same-named values in a managed project's .env. This is especially
// important when Stack Manager updates itself: runtime STATE_DIR=/state is an
// in-container path, while the host compose project uses STATE_DIR=.stack-manager.
func projectComposeEnv(project *Project) []string {
	projectKeys := readDotEnvKeys(filepath.Join(project.Dir, ".env"))
	env := make([]string, 0, len(os.Environ())+len(composeUserEnvKeys)+1)
	for _, entry := range os.Environ() {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		_, definedByProject := projectKeys[key]
		_, runtimeRequired := composeRuntimeEnvKeys[key]
		if definedByProject && !runtimeRequired {
			continue
		}
		if key == "COMPOSE_PROGRESS" {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "COMPOSE_PROGRESS=plain")
	env = append(env, stackManagerUserEnv(project)...)
	return env
}

func stackManagerUserEnv(project *Project) []string {
	if project == nil {
		return nil
	}
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	values := map[string]string{
		"STACK_UID": uid,
		"STACK_GID": gid,
		"PUID":      uid,
		"PGID":      gid,
		"UID":       uid,
		"GID":       gid,
		"USER_UID":  uid,
		"USER_GID":  gid,
	}
	projectEnv := readDotEnvKeys(filepath.Join(project.Dir, ".env"))
	out := make([]string, 0, len(composeUserEnvKeys))
	for _, key := range composeUserEnvKeys {
		if _, ok := projectEnv[key]; ok {
			continue
		}
		out = append(out, key+"="+values[key])
	}
	return out
}

func composeFileArgs(project *Project) []string {
	args := []string{"-f", project.ComposeFile}
	override := filepath.Join(project.Dir, "compose.override.yml")
	if info, err := os.Stat(override); err == nil && !info.IsDir() {
		args = append(args, "-f", override)
	}
	return args
}

func ComposeCommandArgs(project *Project, args ...string) []string {
	composeArgs := []string{"compose"}
	composeArgs = append(composeArgs, composeFileArgs(project)...)
	composeArgs = append(composeArgs, args...)
	return composeArgs
}

func ComposeUserEnv(project *Project) []string {
	return stackManagerUserEnv(project)
}

func readDotEnvKeys(path string) map[string]struct{} {
	keys := map[string]struct{}{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return keys
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}
