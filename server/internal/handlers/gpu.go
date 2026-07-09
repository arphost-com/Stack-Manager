package handlers

import (
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type gpuInfo struct {
	Available bool   `json:"available"`
	Runtime   string `json:"runtime"`
	GPUName   string `json:"gpu_name,omitempty"`
	Driver    string `json:"driver,omitempty"`
}

var (
	gpuCache     *gpuInfo
	gpuCacheTime time.Time
	gpuMu        sync.Mutex
)

func GPUDetect(w http.ResponseWriter, r *http.Request) {
	info := detectGPU()
	writeJSON(w, http.StatusOK, info)
}

func detectGPU() gpuInfo {
	gpuMu.Lock()
	defer gpuMu.Unlock()
	if gpuCache != nil && time.Since(gpuCacheTime) < 5*time.Minute {
		return *gpuCache
	}

	info := gpuInfo{}

	// Check docker info for nvidia runtime
	out, err := exec.Command("docker", "info", "--format", "{{.Runtimes}}").Output()
	if err == nil && strings.Contains(string(out), "nvidia") {
		info.Available = true
		info.Runtime = "nvidia"
	}

	// Try nvidia-smi for GPU details
	if info.Available {
		smi, err := exec.Command("nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader,nounits").Output()
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(smi)), ",", 2)
			if len(parts) >= 1 {
				info.GPUName = strings.TrimSpace(parts[0])
			}
			if len(parts) >= 2 {
				info.Driver = strings.TrimSpace(parts[1])
			}
		}
	}

	gpuCache = &info
	gpuCacheTime = time.Now()
	return info
}

// GPUComposeSnippet returns the deploy section to add to a service
// for GPU access. Empty string if no GPU available.
func GPUComposeSnippet() string {
	info := detectGPU()
	if !info.Available {
		return ""
	}
	return `    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
`
}
