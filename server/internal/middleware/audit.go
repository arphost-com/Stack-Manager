package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

// AuditRecorder writes an audit_log row for every mutating API call so
// operators can see a per-node history of who did what.
func AuditRecorder(store *storage.Store, node string) func(http.Handler) http.Handler {
	if strings.TrimSpace(node) == "" {
		node = defaultAuditNode()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !mutating(r.Method) || skipAudit(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rec := &auditRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if store == nil {
				return
			}
			actor := "anonymous"
			if user, ok := CurrentUser(r); ok && user.Username != "" {
				actor = user.Username
			}
			action, project, target := parseAuditRoute(r)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, _ = store.InsertAuditEntry(ctx, storage.AuditEntry{
				Node:       node,
				Actor:      actor,
				Action:     action,
				Project:    project,
				Target:     target,
				Success:    rec.status < 400,
				DurationMs: int(time.Since(start).Milliseconds()),
				Details:    auditFailureDetails(rec.status, rec.body.Bytes()),
				RemoteIP:   clientIP(r),
			})
		})
	}
}

func mutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

var skipPrefixes = []string{
	"/api/v1/auth/login",
	"/api/v1/auth/logout",
	"/api/v1/audit",
	"/api/v1/metrics/refresh",
}

func skipAudit(path string) bool {
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// parseAuditRoute turns a request into a stable action name and captures the
// project or target ID from chi's route context when present.
func parseAuditRoute(r *http.Request) (action, project, target string) {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		project = rctx.URLParam("name")
		if project == "" {
			project = rctx.URLParam("projectName")
		}
		for _, key := range []string{"jobId", "scheduleID", "agentID", "registry", "skillName", "action", "username"} {
			if v := rctx.URLParam(key); v != "" {
				target = v
				break
			}
		}
	}
	pattern := ""
	if rctx != nil {
		pattern = rctx.RoutePattern()
	}
	action = friendlyAction(r.Method, r.URL.Path, pattern)
	return
}

// friendlyAction prefers chi's matched route PATTERN (e.g.
// "/api/v1/projects/{name}/update") over the concrete path, so every project's
// update collapses to one stable action name that can be filtered as a group.
// Falls back to the concrete path if no pattern is available.
func friendlyAction(method, path, pattern string) string {
	p := pattern
	if strings.TrimSpace(p) == "" || strings.Contains(p, "/*") {
		p = path
	}
	trimmed := strings.TrimPrefix(p, "/api/v1/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return method + " " + trimmed
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		if idx := strings.Index(ip, ","); idx > 0 {
			return strings.TrimSpace(ip[:idx])
		}
		return strings.TrimSpace(ip)
	}
	return r.RemoteAddr
}

func defaultAuditNode() string {
	if name := strings.TrimSpace(os.Getenv("AUDIT_NODE_NAME")); name != "" {
		return name
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		if idx := strings.LastIndex(host, "-server-"); idx > 0 {
			return host[:idx]
		}
		return host
	}
	return "controller"
}

// auditRecorder captures the response status so the middleware can record
// success/failure for each mutating call.
type auditRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
	// body keeps the start of an error response so the audit entry can say
	// why the call failed. Successful responses are not buffered.
	body bytes.Buffer
}

// auditBodyLimit caps how much of an error response is kept for parsing.
const auditBodyLimit = 64 << 10

// auditFailureDetails turns a failed JSON API response into a short reason
// for the audit log. For bulk actions it also names the projects that failed,
// since the request itself carries no single project.
func auditFailureDetails(status int, body []byte) string {
	if status < 400 {
		return ""
	}
	var resp struct {
		Error string `json:"error"`
		Data  struct {
			Results []struct {
				Project string `json:"project"`
				Action  string `json:"action"`
				Success bool   `json:"success"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Error == "" {
		return fmt.Sprintf("HTTP %d", status)
	}
	details := resp.Error
	var failed []string
	for _, r := range resp.Data.Results {
		if !r.Success && r.Project != "" {
			failed = append(failed, r.Project)
		}
	}
	if len(failed) > 0 {
		shown := failed
		if len(shown) > 10 {
			shown = shown[:10]
		}
		details += "; failed: " + strings.Join(shown, ", ")
		if extra := len(failed) - len(shown); extra > 0 {
			details += fmt.Sprintf(" and %d more", extra)
		}
	}
	if len(details) > 500 {
		details = details[:500]
	}
	return details
}

func (a *auditRecorder) WriteHeader(code int) {
	if !a.wrote {
		a.status = code
		a.wrote = true
	}
	a.ResponseWriter.WriteHeader(code)
}

func (a *auditRecorder) Write(b []byte) (int, error) {
	if !a.wrote {
		a.wrote = true
	}
	if a.status >= 400 && a.body.Len() < auditBodyLimit {
		keep := b
		if room := auditBodyLimit - a.body.Len(); len(keep) > room {
			keep = keep[:room]
		}
		a.body.Write(keep)
	}
	return a.ResponseWriter.Write(b)
}
