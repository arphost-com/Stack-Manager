package handlers

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

// SystemAppLogHandler returns Stack Manager's OWN server container logs so the
// operator can see application activity (startup, errors, scheduler runs,
// background metrics) from the dashboard instead of shelling into the host.
// Read-only; the router already gates this behind an authenticated session.
type SystemAppLogHandler struct{}

func NewSystemAppLogHandler() *SystemAppLogHandler { return &SystemAppLogHandler{} }

// selfContainerID returns the current container's identifier. In Docker,
// os.Hostname() is the container's short id unless a hostname is set in compose
// (this repo's server service sets neither), and `docker logs` accepts it.
func selfContainerID() string {
	if h, err := os.Hostname(); err == nil {
		if h = strings.TrimSpace(h); h != "" {
			return h
		}
	}
	return ""
}

// serverContainerByLabel is a fallback used only if the hostname lookup fails:
// find the running container for this stack's server service by compose label.
func serverContainerByLabel() string {
	res, err := core.DockerExec("ps", "--no-trunc", "--filter",
		"label=com.docker.compose.service=server", "--format", "{{.ID}}")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			return id // first match; single-server stacks have exactly one
		}
	}
	return ""
}

// Get streams recent lines from the server container's log. Query param `tail`
// (1..5000, default 500) bounds how many lines are returned.
func (h *SystemAppLogHandler) Get(w http.ResponseWriter, r *http.Request) {
	tail := 500
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}

	cid := selfContainerID()
	res, err := core.DockerExec("logs", "--tail", strconv.Itoa(tail), "--timestamps", cid)
	if err != nil || cid == "" {
		// Hostname lookup failed or isn't a container id — try the compose label.
		if id := serverContainerByLabel(); id != "" && id != cid {
			cid = id
			res, err = core.DockerExec("logs", "--tail", strconv.Itoa(tail), "--timestamps", cid)
		}
	}
	if err != nil {
		msg := "docker logs failed"
		if res != nil && strings.TrimSpace(res.Stderr) != "" {
			msg = strings.TrimSpace(res.Stderr)
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}

	// docker logs sends the container's stdout and stderr on the matching
	// streams. Go's logger writes to stderr, so the interesting lines are in
	// res.Stderr; merge both and sort by the --timestamps prefix (RFC3339Nano,
	// which is lexically chronological) so the view reads top-to-bottom in time.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"container": cid,
		"tail":      tail,
		"log":       mergeTimestampedLogs(res.Stdout, res.Stderr),
	})
}

func mergeTimestampedLogs(stdout, stderr string) string {
	var lines []string
	for _, chunk := range []string{stdout, stderr} {
		for _, ln := range strings.Split(strings.TrimRight(chunk, "\n"), "\n") {
			if strings.TrimSpace(ln) != "" {
				lines = append(lines, ln)
			}
		}
	}
	sort.SliceStable(lines, func(i, j int) bool {
		return firstField(lines[i]) < firstField(lines[j])
	})
	return strings.Join(lines, "\n")
}

func firstField(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
