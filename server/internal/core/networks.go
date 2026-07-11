package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// NetworkInfo describes one Docker network that belongs to a Compose project.
type NetworkInfo struct {
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Scope   string `json:"scope"`
	// InUseBy lists containers currently attached to the network. Docker refuses
	// to remove a network with active endpoints.
	InUseBy []string `json:"in_use_by"`
}

// ListProjectNetworks returns the Docker networks labelled for this Compose
// project. Like volumes, only networks Compose created for the project are
// returned, so unrelated host networks are never exposed or removable.
func (e *Engine) ListProjectNetworks(project *Project) ([]NetworkInfo, error) {
	pname := e.getProjectName(project.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "network", "ls",
		"--filter", "label=com.docker.compose.project="+pname,
		"--format", "{{.Name}}").Output()
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	var nets []NetworkInfo
	for _, name := range strings.Fields(string(out)) {
		ni := NetworkInfo{Name: name}
		if insp, ierr := exec.CommandContext(ctx, "docker", "network", "inspect", name,
			"--format", "{{.Driver}}\t{{.Scope}}").Output(); ierr == nil {
			if parts := strings.SplitN(strings.TrimSpace(string(insp)), "\t", 2); len(parts) == 2 {
				ni.Driver, ni.Scope = parts[0], parts[1]
			}
		}
		if cs, cerr := exec.CommandContext(ctx, "docker", "network", "inspect", name,
			"--format", "{{range .Containers}}{{.Name}} {{end}}").Output(); cerr == nil {
			ni.InUseBy = strings.Fields(string(cs))
		}
		nets = append(nets, ni)
	}
	return nets, nil
}

// RemoveProjectNetwork deletes a single network after confirming it belongs to
// this project. Docker refuses networks with attached containers, and that
// error is returned verbatim.
func (e *Engine) RemoveProjectNetwork(project *Project, networkName string) error {
	nets, err := e.ListProjectNetworks(project)
	if err != nil {
		return err
	}
	found := false
	for _, n := range nets {
		if n.Name == networkName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("network %q is not part of project %q", networkName, project.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "network", "rm", networkName).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
