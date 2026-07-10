package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
)

// OS update runs base-OS package management (apt) on the host through the same
// privileged chroot /host container the GPU helper uses. Admin-only routes.
const (
	osHelperPath   = "/usr/local/sbin/stack-manager-os"
	osHelperImage  = "alpine:3.22"
	osQuickTimeout = 3 * time.Minute
	osLongTimeout  = 30 * time.Minute
)

// osTokenRe bounds package/search terms to a safe apt-friendly charset before
// they ever reach the host. The helper re-checks as defense in depth.
var osTokenRe = regexp.MustCompile(`^[A-Za-z0-9._+:/~-]{1,128}$`)

var errOSHelperMissing = errors.New("host helper not installed at " + osHelperPath)

type OSUpdateHandler struct {
	baseImagePrefix string
}

func NewOSUpdateHandler() *OSUpdateHandler {
	return &OSUpdateHandler{baseImagePrefix: os.Getenv("BASE_IMAGE_PREFIX")}
}

func (h *OSUpdateHandler) runHelper(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dockerArgs := []string{
		"run", "--rm",
		"--privileged",
		"--network=host",
		"--pid=host",
		"-v", "/:/host",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		h.baseImagePrefix + osHelperImage,
		"chroot", "/host",
		osHelperPath,
	}
	dockerArgs = append(dockerArgs, args...)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...) //nolint:gosec // helper path is a constant; token args are validated
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		combined := stderr.String() + " " + out
		if strings.Contains(combined, osHelperPath) &&
			(strings.Contains(combined, "No such file") || strings.Contains(combined, "not found") || strings.Contains(combined, "can't execute")) {
			return out, errOSHelperMissing
		}
		return out + "\n" + stderr.String(), err
	}
	return out, nil
}

func osHelperHint() string {
	return "One-time host setup:\nsudo install -m 750 scripts/stack-manager-os.sh " + osHelperPath
}

func (h *OSUpdateHandler) respond(w http.ResponseWriter, out string, err error) {
	if errors.Is(err, errOSHelperMissing) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"helper_installed": false, "helper_hint": osHelperHint()})
		return
	}
	res := map[string]interface{}{"helper_installed": true, "output": out, "success": err == nil}
	if err != nil {
		res["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, res)
}

// Status runs apt-get update and lists upgradable packages.
func (h *OSUpdateHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	out, err := h.runHelper(r.Context(), osQuickTimeout, "status")
	h.respond(w, out, err)
}

// Upgrade runs update + dist-upgrade + autoremove.
func (h *OSUpdateHandler) Upgrade(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	out, err := h.runHelper(r.Context(), osLongTimeout, "upgrade")
	h.respond(w, out, err)
}

// Autoremove removes unused packages only.
func (h *OSUpdateHandler) Autoremove(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	out, err := h.runHelper(r.Context(), osLongTimeout, "autoremove")
	h.respond(w, out, err)
}

// Search is read-only apt-cache search.
func (h *OSUpdateHandler) Search(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if !osTokenRe.MatchString(term) {
		writeError(w, http.StatusBadRequest, "invalid search term (allowed: letters, digits, . _ + - : / ~)")
		return
	}
	out, err := h.runHelper(r.Context(), osQuickTimeout, "search", term)
	h.respond(w, out, err)
}

// Install installs one apt package.
func (h *OSUpdateHandler) Install(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	var req struct {
		Package string `json:"package"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Package = strings.TrimSpace(req.Package)
	if !osTokenRe.MatchString(req.Package) {
		writeError(w, http.StatusBadRequest, "invalid package name (allowed: letters, digits, . _ + - : / ~)")
		return
	}
	out, err := h.runHelper(r.Context(), osLongTimeout, "install", req.Package)
	h.respond(w, out, err)
}
