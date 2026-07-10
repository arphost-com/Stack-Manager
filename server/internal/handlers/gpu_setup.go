package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GPU setup runs the host helper script (scripts/stack-manager-gpu.sh, installed
// at gpuHelperPath) to install the NVIDIA driver + container toolkit on the
// host. The server runs inside a container, so — exactly like the firewall
// skill — it spawns a throwaway privileged container over the Docker socket,
// bind-mounts the host root, and chroots in to run the helper with host
// binaries, host PID namespace (for systemctl/reboot) and host network (apt).
const (
	gpuHelperPath    = "/usr/local/sbin/stack-manager-gpu"
	gpuHelperImage   = "alpine:3.22"
	gpuStatusTimeout = 30 * time.Second
	gpuInstallTOut   = 20 * time.Minute
)

var errGPUHelperMissing = errors.New("host helper not installed at " + gpuHelperPath)

// GPUSetupHandler carries the base-image prefix so the helper container image
// can route through the GitLab dependency proxy on ARPHost hosts.
type GPUSetupHandler struct {
	baseImagePrefix string
}

func NewGPUSetupHandler() *GPUSetupHandler {
	return &GPUSetupHandler{baseImagePrefix: os.Getenv("BASE_IMAGE_PREFIX")}
}

func (h *GPUSetupHandler) runHelper(ctx context.Context, timeout time.Duration, sub string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dockerArgs := []string{
		"run", "--rm",
		"--privileged",
		"--network=host",
		"--pid=host",
		"-v", "/:/host",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		h.baseImagePrefix + gpuHelperImage,
		"chroot", "/host",
		gpuHelperPath, sub,
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...) //nolint:gosec // helper path + sub are hardcoded constants
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		combined := stderr.String() + " " + out
		if strings.Contains(combined, gpuHelperPath) &&
			(strings.Contains(combined, "No such file") || strings.Contains(combined, "not found") || strings.Contains(combined, "can't execute")) {
			return out, errGPUHelperMissing
		}
		return out + "\n" + stderr.String(), err
	}
	return out, nil
}

func gpuHelperHint() string {
	return "One-time host setup:\nsudo install -m 750 scripts/stack-manager-gpu.sh " + gpuHelperPath
}

// Status reports the host's driver/toolkit/runtime/secure-boot state.
func (h *GPUSetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	out, err := h.runHelper(r.Context(), gpuStatusTimeout, "status")
	if errors.Is(err, errGPUHelperMissing) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"helper_installed": false,
			"helper_hint":      gpuHelperHint(),
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(out))
		return
	}
	writeJSON(w, http.StatusOK, parseGPUStatus(out))
}

// Install installs the driver + toolkit and configures the Docker runtime. A
// reboot (and, under Secure Boot, a console MOK enrollment) is required after.
func (h *GPUSetupHandler) Install(w http.ResponseWriter, r *http.Request) {
	out, err := h.runHelper(r.Context(), gpuInstallTOut, "install")
	if errors.Is(err, errGPUHelperMissing) {
		writeError(w, http.StatusBadRequest, gpuHelperHint())
		return
	}
	status := map[string]interface{}{"output": out, "success": err == nil}
	if err != nil {
		status["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, status)
}

// Uninstall removes the driver + toolkit (used to re-test the flow).
func (h *GPUSetupHandler) Uninstall(w http.ResponseWriter, r *http.Request) {
	out, err := h.runHelper(r.Context(), gpuInstallTOut, "uninstall")
	if errors.Is(err, errGPUHelperMissing) {
		writeError(w, http.StatusBadRequest, gpuHelperHint())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"output": out, "success": err == nil})
}

// Reboot reboots the host so the NVIDIA kernel module can load.
func (h *GPUSetupHandler) Reboot(w http.ResponseWriter, r *http.Request) {
	out, err := h.runHelper(r.Context(), gpuStatusTimeout, "reboot")
	if errors.Is(err, errGPUHelperMissing) {
		writeError(w, http.StatusBadRequest, gpuHelperHint())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"output": out, "success": err == nil, "rebooting": err == nil})
}

// parseGPUStatus turns the helper's key=value lines into a structured object.
func parseGPUStatus(out string) map[string]interface{} {
	res := map[string]interface{}{"helper_installed": true, "raw": out}
	gpus := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if k == "gpu" {
			gpus = append(gpus, v)
			continue
		}
		res[k] = v
	}
	res["gpus"] = gpus
	return res
}
