package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// fillHostNetworkPorts sets HostPorts on every host-network container in the
// project. Docker publishes nothing for these containers, which left the UI
// link, the firewall action and Add to proxy with no port to use. The ports
// come from numeric *_PORT values in the project's .env, or from the TCP ports
// the container's image exposes when the .env declares none. exposed is
// injectable for tests.
func fillHostNetworkPorts(p *Project, exposed func(containerID string) []int) {
	var envPorts []int
	envLoaded := false
	for i := range p.Containers {
		c := &p.Containers[i]
		if !c.HostNetwork {
			continue
		}
		if !envLoaded {
			envPorts = envDeclaredPorts(filepath.Join(p.Dir, ".env"))
			envLoaded = true
		}
		// With host networking the app listens where its .env tells it to, so
		// those ports win. The image's EXPOSE list often names the default
		// ports the .env has moved away from, so it is only a fallback for
		// projects whose .env declares no ports.
		ports := envPorts
		if len(ports) == 0 && exposed != nil && c.ID != "" {
			ports = exposed(c.ID)
		}
		c.HostPorts = uniqueSortedPorts(ports)
	}
}

// envDeclaredPorts returns the values of KEY_PORT=<number> lines in an env
// file. Keys must end in PORT so unrelated numbers (UIDs, sizes) are ignored.
func envDeclaredPorts(path string) []int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var ports []int
	for key, value := range parseEnvMap(string(data)) {
		if !strings.HasSuffix(strings.ToUpper(key), "PORT") {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if port, err := strconv.Atoi(value); err == nil && port > 0 && port <= 65535 {
			ports = append(ports, port)
		}
	}
	return ports
}

// imageExposedTCPPorts reads the container's exposed TCP ports from Docker.
func imageExposedTCPPorts(containerID string) []int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Config.ExposedPorts}}", containerID).Output()
	if err != nil {
		return nil
	}
	return parseExposedTCPPorts(out)
}

func parseExposedTCPPorts(raw []byte) []int {
	var exposed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &exposed); err != nil {
		return nil
	}
	var ports []int
	for spec := range exposed {
		num, proto, _ := strings.Cut(spec, "/")
		if proto != "" && proto != "tcp" {
			continue
		}
		if port, err := strconv.Atoi(num); err == nil && port > 0 && port <= 65535 {
			ports = append(ports, port)
		}
	}
	return ports
}

func uniqueSortedPorts(ports []int) []int {
	if len(ports) == 0 {
		return nil
	}
	seen := make(map[int]bool, len(ports))
	out := make([]int, 0, len(ports))
	for _, port := range ports {
		if !seen[port] {
			seen[port] = true
			out = append(out, port)
		}
	}
	sort.Ints(out)
	return out
}
