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

// SystemTZHandler sets the HOST system timezone (Debian + Ubuntu) through a
// scoped root helper on the host, using the same privileged chroot /host pattern
// as the GPU/OS/self-update handlers. Persisting the TZ app setting (so the
// scheduler/UI use it) is handled separately by EnvSettingsHandler; this applies
// it to the host clock.
const (
	tzHelperPath  = "/usr/local/sbin/stack-manager-tz"
	tzHelperImage = "alpine:3.22"
	tzTimeout     = 30 * time.Second
)

var errTZHelperMissing = errors.New("host helper not installed at " + tzHelperPath)

// tzNameRe mirrors the helper's validation: IANA zone names only. Defense in
// depth so nothing shell-unsafe is passed to the helper.
var tzNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_+/-]*$`)

type SystemTZHandler struct {
	baseImagePrefix string
}

func NewSystemTZHandler() *SystemTZHandler {
	return &SystemTZHandler{baseImagePrefix: os.Getenv("BASE_IMAGE_PREFIX")}
}

func (h *SystemTZHandler) runHelper(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, tzTimeout)
	defer cancel()
	dockerArgs := append([]string{
		"run", "--rm",
		"--privileged",
		"-v", "/:/host",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		h.baseImagePrefix + tzHelperImage,
		"chroot", "/host",
		tzHelperPath,
	}, args...)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...) //nolint:gosec // helper path is constant; tz arg is regex-validated
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		combined := stderr.String() + " " + out
		if strings.Contains(combined, tzHelperPath) &&
			(strings.Contains(combined, "No such file") || strings.Contains(combined, "not found") || strings.Contains(combined, "can't execute")) {
			return out, errTZHelperMissing
		}
		return out + "\n" + stderr.String(), err
	}
	return out, nil
}

func tzHelperHint() string {
	return "One-time host setup:\nsudo install -m 750 scripts/stack-manager-tz.sh " + tzHelperPath
}

// parseKV pulls the value of a `key=...` line out of helper output.
func parseKV(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Status reports whether the helper is installed and the host's current zone.
func (h *SystemTZHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	out, err := h.runHelper(r.Context(), "get")
	if errors.Is(err, errTZHelperMissing) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"helper_installed": false, "helper_hint": tzHelperHint()})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(out))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"helper_installed": true, "tz": parseKV(out, "tz")})
}

// Apply sets the host system timezone.
func (h *SystemTZHandler) Apply(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	var req struct {
		TZ string `json:"tz"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tz := strings.TrimSpace(req.TZ)
	if !tzNameRe.MatchString(tz) {
		writeError(w, http.StatusBadRequest, "invalid timezone name")
		return
	}
	out, err := h.runHelper(r.Context(), "set", tz)
	if errors.Is(err, errTZHelperMissing) {
		writeError(w, http.StatusBadRequest, tzHelperHint())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, strings.TrimSpace(out))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tz": parseKV(out, "ok"), "output": strings.TrimSpace(out)})
}
