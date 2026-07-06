package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Per-image manifest cache. Manifest inspect results are effectively static
// (a tag pinned in compose.yaml points at the same digest for weeks or
// months), so caching for a full day keeps the "public / private / not
// found" access column fresh without turning every Dashboard render into a
// wave of Docker Hub pulls. Restarting the server clears the cache, which
// is fine because the miss path just re-issues the inspects.
type manifestCacheEntry struct {
	ok      bool
	message string
	expires time.Time
}

var (
	manifestCacheMu   sync.RWMutex
	manifestCache     = map[string]manifestCacheEntry{}
	manifestCacheTTL  = 24 * time.Hour
)

func manifestCacheKey(image string, anonymous bool) string {
	if anonymous {
		return "anon:" + image
	}
	return "auth:" + image
}

func manifestCacheGet(image string, anonymous bool) (manifestCacheEntry, bool) {
	manifestCacheMu.RLock()
	defer manifestCacheMu.RUnlock()
	entry, ok := manifestCache[manifestCacheKey(image, anonymous)]
	if !ok || time.Now().After(entry.expires) {
		return manifestCacheEntry{}, false
	}
	return entry, true
}

func manifestCacheSet(image string, anonymous, ok bool, message string) {
	manifestCacheMu.Lock()
	defer manifestCacheMu.Unlock()
	manifestCache[manifestCacheKey(image, anonymous)] = manifestCacheEntry{
		ok:      ok,
		message: message,
		expires: time.Now().Add(manifestCacheTTL),
	}
}

// ImageSources returns compose service image metadata without contacting registries.
func (e *Engine) ImageSources(project *Project) []ImageSource {
	sources, err := readComposeImageSources(project)
	if err != nil {
		return nil
	}
	return sources
}

// CheckImageSources returns service image metadata and probes registry access.
// Optimized to hit Docker Hub as little as possible: when a saved auth exists
// for the image's registry we skip the anonymous probe entirely, so a logged
// in user consumes ~half as many pulls as before. Every result is cached in
// process for 24 hours (see manifestCache).
func (e *Engine) CheckImageSources(project *Project) []ImageSource {
	sources := e.ImageSources(project)
	for i := range sources {
		if sources[i].SourceType == "custom" {
			sources[i].Access = "local-build"
			sources[i].Message = "service is built from the compose project"
			continue
		}
		if sources[i].Image == "" {
			sources[i].Access = "unknown"
			sources[i].Message = "service has no image reference"
			continue
		}

		if HasStoredAuthForRegistry(sources[i].Registry) {
			// Logged in for this registry: authenticated probe is enough,
			// skip the anonymous call so we do not double-charge the pull
			// budget just to detect "is this image public".
			authOK, authMsg := manifestInspect(sources[i].Image, false)
			if authOK {
				sources[i].Access = "authenticated"
				sources[i].Message = "registry manifest is reachable with saved credentials"
				continue
			}
			sources[i].Access = classifyRegistryFailure(authMsg)
			sources[i].Message = strings.TrimSpace(authMsg)
			continue
		}

		// No stored auth for this registry, so an anonymous check is the
		// only signal we have. On success we know it is public; on failure
		// the user needs to log in.
		anonymousOK, anonymousMsg := manifestInspect(sources[i].Image, true)
		if anonymousOK {
			sources[i].Access = "public"
			sources[i].Message = "registry manifest is reachable without credentials"
			continue
		}
		sources[i].Access = classifyRegistryFailure(anonymousMsg)
		sources[i].Message = strings.TrimSpace(anonymousMsg)
	}
	return sources
}

func readComposeImageSources(project *Project) ([]ImageSource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", project.ComposeFile, "config", "--format", "json")
	cmd.Dir = project.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("compose config failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var cfg struct {
		Services map[string]struct {
			Image string      `json:"image"`
			Build interface{} `json:"build"`
		} `json:"services"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &cfg); err != nil {
		return nil, err
	}

	sources := make([]ImageSource, 0, len(cfg.Services))
	for service, svc := range cfg.Services {
		source := ImageSource{
			Service: service,
			Image:   svc.Image,
		}
		if svc.Build != nil {
			source.Build = true
			source.BuildContext = buildContext(svc.Build)
			source.SourceType = "custom"
		} else if svc.Image != "" {
			source.SourceType = "registry"
		} else {
			source.SourceType = "unknown"
		}

		if svc.Image != "" {
			source.Registry, source.Repository, source.Tag = parseImageReference(svc.Image)
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func buildContext(build interface{}) string {
	switch v := build.(type) {
	case string:
		return v
	case map[string]interface{}:
		if ctx, ok := v["context"].(string); ok {
			return ctx
		}
	}
	return ""
}

func parseImageReference(image string) (registry, repository, tag string) {
	ref := image
	if digestIdx := strings.Index(ref, "@"); digestIdx >= 0 {
		ref = ref[:digestIdx]
	}

	parts := strings.Split(ref, "/")
	if len(parts) > 1 && isRegistryHost(parts[0]) {
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	} else {
		registry = "docker.io"
		repository = ref
	}

	lastSlash := strings.LastIndex(repository, "/")
	lastColon := strings.LastIndex(repository, ":")
	if lastColon > lastSlash {
		tag = repository[lastColon+1:]
		repository = repository[:lastColon]
	} else {
		tag = "latest"
	}

	if registry == "docker.io" && !strings.Contains(repository, "/") {
		repository = "library/" + repository
	}
	return registry, repository, tag
}

func isRegistryHost(part string) bool {
	return part == "localhost" || strings.Contains(part, ".") || strings.Contains(part, ":")
}

func manifestInspect(image string, anonymous bool) (bool, string) {
	if entry, ok := manifestCacheGet(image, anonymous); ok {
		return entry.ok, entry.message
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "manifest", "inspect", image)
	if anonymous {
		tmpDir, err := os.MkdirTemp("", "stack-manager-docker-config-*")
		if err != nil {
			return false, err.Error()
		}
		defer os.RemoveAll(tmpDir)
		if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{"auths":{}}`), 0600); err != nil {
			return false, err.Error()
		}
		cmd.Env = append(cmd.Environ(), "DOCKER_CONFIG="+tmpDir)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		manifestCacheSet(image, anonymous, true, "")
		return true, ""
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = strings.TrimSpace(stdout.String())
	}
	if msg == "" {
		msg = err.Error()
	}
	manifestCacheSet(image, anonymous, false, msg)
	return false, msg
}

func classifyRegistryFailure(messages ...string) string {
	combined := strings.ToLower(strings.Join(messages, "\n"))
	switch {
	case strings.Contains(combined, "unauthorized"),
		strings.Contains(combined, "authentication required"),
		strings.Contains(combined, "denied"),
		strings.Contains(combined, "insufficient_scope"):
		return "private-login-required"
	case strings.Contains(combined, "no such manifest"),
		strings.Contains(combined, "manifest unknown"),
		strings.Contains(combined, "not found"):
		return "not-found"
	default:
		return "inaccessible"
	}
}
