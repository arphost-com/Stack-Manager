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

	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
)

// Self-update pulls the latest code and rebuilds the controller's own stack,
// via the same privileged chroot /host helper pattern as GPU/OS setup. The
// rebuild is detached on the host so it survives the server/web containers
// restarting mid-update.
const (
	updateHelperPath  = "/usr/local/sbin/stack-manager-update"
	updateHelperImage = "alpine:3.22"
	updateTimeout     = 60 * time.Second
)

var errUpdateHelperMissing = errors.New("host helper not installed at " + updateHelperPath)

type SelfUpdateHandler struct {
	baseImagePrefix string
}

func NewSelfUpdateHandler() *SelfUpdateHandler {
	return &SelfUpdateHandler{baseImagePrefix: os.Getenv("BASE_IMAGE_PREFIX")}
}

func (h *SelfUpdateHandler) runHelper(ctx context.Context, sub string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	dockerArgs := []string{
		"run", "--rm",
		"--privileged",
		"--network=host",
		"--pid=host",
		"-v", "/:/host",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		h.baseImagePrefix + updateHelperImage,
		"chroot", "/host",
		updateHelperPath, sub,
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...) //nolint:gosec // helper path + sub are constants
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		combined := stderr.String() + " " + out
		if strings.Contains(combined, updateHelperPath) &&
			(strings.Contains(combined, "No such file") || strings.Contains(combined, "not found") || strings.Contains(combined, "can't execute")) {
			return out, errUpdateHelperMissing
		}
		return out + "\n" + stderr.String(), err
	}
	return out, nil
}

func updateHelperHint() string {
	return "One-time host setup:\nsudo install -m 750 scripts/stack-manager-update.sh " + updateHelperPath
}

// Status reports the deploy branch and how many commits behind upstream it is.
func (h *SelfUpdateHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	out, err := h.runHelper(r.Context(), "status")
	if errors.Is(err, errUpdateHelperMissing) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"helper_installed": false, "helper_hint": updateHelperHint()})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(out))
		return
	}
	res := map[string]interface{}{"helper_installed": true}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			res[k] = v
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// Update kicks off a detached pull + rebuild of the controller's own stack.
func (h *SelfUpdateHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	out, err := h.runHelper(r.Context(), "update")
	if errors.Is(err, errUpdateHelperMissing) {
		writeError(w, http.StatusBadRequest, updateHelperHint())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(out))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"started": true,
		"output":  strings.TrimSpace(out),
		"note":    "Update is running in the background on the host. The dashboard will briefly restart; reconnect in a few minutes.",
	})
}
