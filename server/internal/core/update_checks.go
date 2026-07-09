package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ImageUpdateStatusAvailable = "update_available"
	ImageUpdateStatusCurrent   = "current"
	ImageUpdateStatusMissing   = "missing_local"
	ImageUpdateStatusSkipped   = "skipped"
	ImageUpdateStatusUnknown   = "unknown"
)

// CheckProjectUpdates compares local image digests to registry manifests
// without pulling images. For git-based projects with no registry images,
// it checks for new commits instead.
func (e *Engine) CheckProjectUpdates(ctx context.Context, project *Project) ProjectUpdateStatus {
	status := ProjectUpdateStatus{Checked: true}
	if project == nil {
		status.Error = "project is nil"
		return status
	}

	// Git-based projects: check for upstream commits instead of registry images.
	if project.IsGit {
		gitStatus := e.checkGitUpdates(ctx, project)
		if gitStatus.Checked {
			return gitStatus
		}
		// Fall through to image check if git check couldn't determine anything.
	}

	sources := project.ImageSources
	if len(sources) == 0 {
		sources = e.ImageSources(project)
	}
	now := time.Now().UTC()
	for _, source := range sources {
		if source.SourceType != "registry" || source.Image == "" {
			status.SkippedServices++
			continue
		}
		status.RegistryImages++
		check := e.CheckImageUpdate(ctx, project.Name, source.Service, source.Image, source.Registry, now)
		status.Images = append(status.Images, check)
		if check.UpdateAvailable {
			status.Available = true
			status.Count++
		}
		if check.Error != "" && status.Error == "" {
			status.Error = check.Error
		}
	}
	if len(status.Images) > 0 {
		status.CheckedAt = &now
	}
	return status
}

// checkGitUpdates runs git fetch --dry-run and compares HEAD to the
// upstream branch to see if there are new commits.
func (e *Engine) checkGitUpdates(ctx context.Context, project *Project) ProjectUpdateStatus {
	status := ProjectUpdateStatus{Checked: true}
	now := time.Now().UTC()
	status.CheckedAt = &now

	// git fetch to update remote refs.
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	fetchCmd := exec.CommandContext(fetchCtx, "git", "fetch", "--all", "--prune")
	fetchCmd.Dir = project.Dir
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		status.Error = "git fetch failed: " + strings.TrimSpace(string(out))
		return status
	}

	// Count commits behind upstream.
	behindCmd := exec.CommandContext(ctx, "git", "rev-list", "--count", "HEAD..@{upstream}")
	behindCmd.Dir = project.Dir
	out, err := behindCmd.Output()
	if err != nil {
		// No upstream tracking branch — not an error, just can't determine.
		status.Checked = false
		return status
	}

	behind := strings.TrimSpace(string(out))
	if behind != "0" && behind != "" {
		n, _ := strconv.Atoi(behind)
		status.Available = true
		status.Count = n
		status.Images = append(status.Images, ImageUpdateCheck{
			Project:         project.Name,
			Service:         "git",
			Image:           "git repository",
			Status:          behind + " commits behind",
			UpdateAvailable: true,
			CheckedAt:       now,
		})
	}
	return status
}

// CheckImageUpdate compares one image reference against the registry without mutating local images.
func (e *Engine) CheckImageUpdate(ctx context.Context, project, service, image, registry string, checkedAt time.Time) ImageUpdateCheck {
	check := ImageUpdateCheck{
		Project:   project,
		Service:   service,
		Image:     image,
		Status:    ImageUpdateStatusUnknown,
		CheckedAt: checkedAt.UTC(),
	}
	local, localErr := localImageRepoDigest(ctx, image)
	if localErr != nil {
		check.Status = ImageUpdateStatusMissing
		check.UpdateAvailable = true
		check.Error = localErr.Error()
	}
	remote, remoteErr := remoteImageDigest(ctx, image, HasStoredAuthForRegistry(registry))
	if remoteErr != nil {
		check.Error = remoteErr.Error()
		if local != "" {
			check.LocalDigest = local
		}
		return check
	}
	check.RemoteDigest = remote
	if local == "" {
		check.Status = ImageUpdateStatusMissing
		check.UpdateAvailable = true
		return check
	}
	check.LocalDigest = local
	if digestMatches(local, remote) {
		check.Status = ImageUpdateStatusCurrent
		return check
	}
	check.Status = ImageUpdateStatusAvailable
	check.UpdateAvailable = true
	return check
}

func localImageRepoDigest(ctx context.Context, image string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{json .RepoDigests}}", image)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("local image not found: %s", msg)
	}
	var digests []string
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &digests); err != nil {
		return "", err
	}
	if len(digests) == 0 {
		return "", fmt.Errorf("local image has no repo digest")
	}
	for _, digest := range digests {
		if strings.Contains(digest, "@sha256:") {
			return digest, nil
		}
	}
	return digests[0], nil
}

func remoteImageDigest(ctx context.Context, image string, authenticated bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "manifest", "inspect", "--verbose", image)
	if !authenticated {
		tmpDir, cleanup, err := anonymousDockerConfig()
		if err == nil {
			defer cleanup()
			cmd.Env = append(cmd.Environ(), "DOCKER_CONFIG="+tmpDir)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("remote manifest unavailable: %s", msg)
	}
	digest := digestFromManifest(stdout.Bytes())
	if digest == "" {
		return "", fmt.Errorf("remote manifest digest unavailable")
	}
	return digest, nil
}

func anonymousDockerConfig() (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "stack-manager-docker-config-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(`{"auths":{}}`), 0600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return tmpDir, cleanup, nil
}

func digestFromManifest(raw []byte) string {
	var list []manifestInspectResult
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, item := range list {
			if digest := item.digest(); digest != "" {
				return digest
			}
		}
	}

	var item manifestInspectResult
	if err := json.Unmarshal(raw, &item); err != nil {
		return ""
	}
	return item.digest()
}

type manifestInspectResult struct {
	Descriptor manifestDescriptor `json:"Descriptor"`
	Digest     string             `json:"digest"`
	DigestAlt  string             `json:"Digest"`
}

func (r manifestInspectResult) digest() string {
	for _, digest := range []string{r.Descriptor.Digest, r.Descriptor.DigestAlt, r.Digest, r.DigestAlt} {
		if strings.HasPrefix(digest, "sha256:") {
			return digest
		}
	}
	return ""
}

type manifestDescriptor struct {
	Digest    string `json:"digest"`
	DigestAlt string `json:"Digest"`
}

func digestMatches(local, remote string) bool {
	if local == "" || remote == "" {
		return false
	}
	localDigest := local
	if idx := strings.LastIndex(localDigest, "@"); idx >= 0 {
		localDigest = localDigest[idx+1:]
	}
	return localDigest == remote
}
