package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// VolumeInfo describes one Docker volume that belongs to a Compose project.
type VolumeInfo struct {
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Mountpoint string   `json:"mountpoint"`
	CreatedAt  string   `json:"created_at"`
	// InUseBy lists containers currently mounting the volume. A non-empty list
	// means Docker will refuse to remove it until those containers are gone.
	InUseBy []string `json:"in_use_by"`
}

// ListProjectVolumes returns the Docker volumes labelled for this Compose
// project (com.docker.compose.project=<name>). Only volumes Compose created for
// the project are returned, so a caller can never see or act on unrelated
// volumes on the host.
func (e *Engine) ListProjectVolumes(project *Project) ([]VolumeInfo, error) {
	pname := e.getProjectName(project.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "volume", "ls",
		"--filter", "label=com.docker.compose.project="+pname,
		"--format", "{{.Name}}").Output()
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}

	var vols []VolumeInfo
	for _, name := range strings.Fields(string(out)) {
		vi := VolumeInfo{Name: name}
		if insp, ierr := exec.CommandContext(ctx, "docker", "volume", "inspect", name,
			"--format", "{{.Driver}}\t{{.Mountpoint}}\t{{.CreatedAt}}").Output(); ierr == nil {
			if parts := strings.SplitN(strings.TrimSpace(string(insp)), "\t", 3); len(parts) == 3 {
				vi.Driver, vi.Mountpoint, vi.CreatedAt = parts[0], parts[1], parts[2]
			}
		}
		if ps, perr := exec.CommandContext(ctx, "docker", "ps", "-a",
			"--filter", "volume="+name, "--format", "{{.Names}}").Output(); perr == nil {
			vi.InUseBy = strings.Fields(string(ps))
		}
		vols = append(vols, vi)
	}
	return vols, nil
}

// RemoveProjectVolume deletes a single volume, but only after confirming it
// belongs to this project — so a crafted volume name can never delete something
// outside the selected stack. Docker refuses volumes still in use, and that
// error is returned verbatim.
func (e *Engine) RemoveProjectVolume(project *Project, volumeName string) error {
	vols, err := e.ListProjectVolumes(project)
	if err != nil {
		return err
	}
	found := false
	for _, v := range vols {
		if v.Name == volumeName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("volume %q is not part of project %q", volumeName, project.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "volume", "rm", volumeName).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
