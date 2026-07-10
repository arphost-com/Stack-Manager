package handlers

import (
	"context"
	"net/http"
	"os"
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

type gpuTestResult struct {
	Success bool   `json:"success"`
	Image   string `json:"image"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// GPUTest runs a throwaway container with --gpus all and nvidia-smi to prove the
// full passthrough chain (driver + nvidia-container-toolkit + runtime) actually
// works end-to-end, not just that the runtime is registered. The first run pulls
// the CUDA base image, so allow a generous timeout. The image is fixed (env
// override only), never user input, and the call runs without a shell.
func GPUTest(w http.ResponseWriter, r *http.Request) {
	image := strings.TrimSpace(os.Getenv("GPU_TEST_IMAGE"))
	if image == "" {
		image = "nvidia/cuda:12.4.1-base-ubuntu22.04"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--gpus", "all", image, "nvidia-smi")
	out, err := cmd.CombinedOutput()
	res := gpuTestResult{Image: image, Output: strings.TrimRight(string(out), "\n")}
	if err != nil {
		res.Success = false
		res.Error = err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			res.Error = "timed out (image pull or run took too long)"
		}
	} else {
		res.Success = true
		// A working run always includes the nvidia-smi header.
		if !strings.Contains(res.Output, "NVIDIA-SMI") {
			res.Success = false
			res.Error = "nvidia-smi did not report a GPU"
		}
	}
	// Refresh detection cache alongside a successful test.
	if res.Success {
		gpuMu.Lock()
		gpuCache = nil
		gpuMu.Unlock()
	}
	writeJSON(w, http.StatusOK, res)
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
