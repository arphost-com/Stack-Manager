// Package firewall drives ConfigServer Firewall (csf/lfd) on the host
// through a narrow root helper script installed at /usr/local/sbin/
// stack-manager-csf on the host. The server itself runs inside a
// container that only has access to the host through the mounted
// /var/run/docker.sock. To reach the host firewall, this package
// spawns a throwaway privileged container via that socket, bind-mounts
// the host root filesystem into it, and chroot's into the host so the
// helper script executes with real host paths and host networking.
// This is the same pattern backup.runTarInRootHelper uses when the
// non-root server user cannot read a project's bind-mounted data.
//
// The helper validates every input against strict patterns, so a
// compromise of the server user still cannot use this path to run
// arbitrary shell — the docker-run command line only carries the
// subcommand name and pre-validated arguments.
package firewall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
	"github.com/go-chi/chi/v5"
)

const (
	defaultHelperPath = "/usr/local/sbin/stack-manager-csf"
	// dockerBinary is a compile-time constant so semgrep's
	// go.lang.security.audit.dangerous-exec-command rule doesn't flag
	// the exec.Command below.
	defaultHelperImage = "alpine:3.22"
	commandTimeout     = 60 * time.Second
	installTimeout     = 10 * time.Minute
)

var allowedConfigs = map[string]struct{}{
	"csf.conf":    {},
	"csf.allow":   {},
	"csf.deny":    {},
	"csf.ignore":  {},
	"csf.pignore": {},
}

// Skill exposes CSF operations over the /api/v1/skills/firewall/ route
// group. It also satisfies auth.IPAllower so the login handler can
// auto-allowlist the caller's IP without holding a direct dependency on
// this package.
type Skill struct {
	helperPath string
	// baseImagePrefix is prepended to the helper container's alpine
	// image reference so hosts routing through the GitLab dependency
	// proxy don't hit Docker Hub rate limits. Empty by default.
	baseImagePrefix string

	mu         sync.Mutex
	lastStatus StatusResult
	statusTime time.Time
}

// New returns an unconfigured Skill. Init picks up overrides.
func New() *Skill {
	return &Skill{helperPath: defaultHelperPath}
}

func (s *Skill) Name() string        { return "firewall" }
func (s *Skill) Description() string { return "ConfigServer Firewall (csf/lfd) install, monitor, and IP management via a narrow root helper" }
func (s *Skill) Version() string     { return "0.1.0" }

func (s *Skill) Init(ctx context.Context, engine *core.Engine, cfg map[string]interface{}) error {
	if v, ok := cfg["firewall_helper_path"].(string); ok && v != "" {
		s.helperPath = v
	}
	if v, ok := cfg["base_image_prefix"].(string); ok && v != "" {
		s.baseImagePrefix = v
	} else if env := os.Getenv("BASE_IMAGE_PREFIX"); env != "" {
		s.baseImagePrefix = env
	}
	return nil
}

func (s *Skill) Shutdown(ctx context.Context) error { return nil }

func (s *Skill) HealthCheck(ctx context.Context) error {
	// The skill is "healthy" even if csf is not installed — that's a
	// state to report, not a failure to serve requests.
	return nil
}

// RegisterRoutes mounts the firewall routes. All routes require admin.
func (s *Skill) RegisterRoutes(r chi.Router) {
	r.Get("/client-ip", s.getClientIP) // available to any signed-in user (used by their own "add my IP" button)
	r.Group(func(gr chi.Router) {
		gr.Use(adminOnly)
		gr.Get("/status", s.handleStatus)
		gr.Get("/version", s.handleVersion)
		gr.Post("/install", s.handleInstall)
		gr.Post("/uninstall", s.handleUninstall)
		gr.Post("/restart", s.handleRestart)
		gr.Post("/reload-lfd", s.handleReloadLFD)
		gr.Get("/allow", s.handleListAllow)
		gr.Get("/deny", s.handleListDeny)
		gr.Get("/tempbans", s.handleListTempbans)
		gr.Post("/ips/allow", s.handleAllowIP)
		gr.Post("/ips/deny", s.handleDenyIP)
		gr.Delete("/ips/{ip}", s.handleRemoveIP)
		gr.Get("/config/{name}", s.handleReadConfig)
		gr.Put("/config/{name}", s.handleWriteConfig)
		gr.Get("/conf-settings", s.handleGetConfSettings)
		gr.Put("/conf-settings", s.handleSaveConfSettings)
		gr.Get("/log", s.handleTailLog)
		gr.Post("/allow-my-ip", s.handleAllowMyIP)
	})
}

func adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !middleware.RequireAdmin(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SyncProjectPorts reads the current TCP_IN from csf.conf, adds any
// host-mapped ports from the given port strings (e.g. "0.0.0.0:8080->80/tcp"),
// and writes back if anything changed. Called after project up/update/create
// so CSF doesn't block newly deployed stacks.
func (s *Skill) SyncProjectPorts(ctx context.Context, portStrings []string) {
	if len(portStrings) == 0 {
		return
	}
	// Parse host ports from Docker port mappings like "0.0.0.0:8080->80/tcp"
	var ports []string
	for _, ps := range portStrings {
		for _, mapping := range strings.Split(ps, ",") {
			mapping = strings.TrimSpace(mapping)
			if idx := strings.Index(mapping, "->"); idx > 0 {
				hostPart := mapping[:idx]
				if colonIdx := strings.LastIndex(hostPart, ":"); colonIdx >= 0 {
					port := strings.TrimSpace(hostPart[colonIdx+1:])
					if port != "" && port != "0" {
						ports = append(ports, port)
					}
				}
			}
		}
	}
	if len(ports) == 0 {
		return
	}

	// Read current TCP_IN, add missing ports, write back + restart
	raw, err := s.runHelper(ctx, commandTimeout, nil, "read-config", "csf.conf")
	if errors.Is(err, errHelperMissing) || err != nil {
		return
	}
	parsed := parseConfKeys(raw, []string{"TCP_IN"})
	currentTCPIn := parsed["TCP_IN"]
	if currentTCPIn == "" {
		return
	}

	currentSet := map[string]bool{}
	for _, p := range strings.Split(currentTCPIn, ",") {
		currentSet[strings.TrimSpace(p)] = true
	}

	var added []string
	for _, p := range ports {
		if !currentSet[p] {
			added = append(added, p)
			currentSet[p] = true
		}
	}
	if len(added) == 0 {
		return
	}

	newTCPIn := currentTCPIn + "," + strings.Join(added, ",")
	updated := strings.ReplaceAll(applyConfChanges(raw, map[string]string{"tcp_in": newTCPIn}), "\x00", "")
	_, _ = s.runHelper(ctx, commandTimeout, strings.NewReader(updated), "write-config", "csf.conf")
	_, _ = s.runHelper(ctx, commandTimeout, nil, "restart")
}

// AllowIP is called by the auth handler after a successful login. It
// swallows errors (they're logged by runHelper stderr) so a firewall
// misconfiguration cannot break login. It also skips loopback / unspec
// addresses so a container-internal request doesn't get whitelisted.
func (s *Skill) AllowIP(ctx context.Context, ip, comment string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("empty ip")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid ip: %q", ip)
	}
	if parsed.IsLoopback() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast() {
		return nil
	}
	comment = sanitizeComment(comment)
	if comment == "" {
		comment = "Stack Manager"
	}
	_, err := s.runHelper(ctx, commandTimeout, nil, "allow-ip", ip, comment)
	if errors.Is(err, errHelperMissing) {
		// Firewall not set up on this host. Login should not be gated
		// on it and repeated logs are noise.
		return nil
	}
	return err
}

// --- HTTP handlers -----------------------------------------------------------

type StatusResult struct {
	Installed         bool   `json:"installed"`
	HelperInstalled   bool   `json:"helper_installed"`
	HelperInstallHint string `json:"helper_install_hint,omitempty"`
	TestingMode       string `json:"testing_mode"`
	IptablesRules     int    `json:"iptables_rules"`
	LFDActive         bool   `json:"lfd_active"`
	Version           string `json:"version"`
	Raw               string `json:"raw"`
}

// errHelperMissing is a sentinel for the "the root helper script is not
// installed on the host" state. The Firewall panel loads /status on
// every visit, and we don't want to greet the user with a red error
// toast when the only thing wrong is that they haven't run the one-time
// install command yet.
var errHelperMissing = errors.New("host helper script not installed at " + defaultHelperPath)

func helperInstallHint() string {
	return "Run this on the host once (one-time setup):\n" +
		"sudo install -m 750 scripts/stack-manager-csf.sh " + defaultHelperPath
}

func (s *Skill) handleStatus(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "status")
	if errors.Is(err, errHelperMissing) {
		writeJSON(w, http.StatusOK, StatusResult{
			Installed:         false,
			HelperInstalled:   false,
			HelperInstallHint: helperInstallHint(),
			Version:           "host helper not installed",
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	vout, _ := s.runHelper(r.Context(), commandTimeout, nil, "version")
	result := parseStatus(out, vout)
	result.HelperInstalled = true
	s.mu.Lock()
	s.lastStatus = result
	s.statusTime = time.Now().UTC()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, result)
}

func (s *Skill) handleVersion(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "version")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": strings.TrimSpace(out)})
}

func (s *Skill) handleInstall(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), installTimeout, nil, "install")
	writeCommandResult(w, out, err)
}

func (s *Skill) handleUninstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.ToUpper(strings.TrimSpace(body.Confirm)) != "UNINSTALL" {
		writeError(w, http.StatusBadRequest, `confirmation required: {"confirm":"UNINSTALL"}`)
		return
	}
	out, err := s.runHelper(r.Context(), installTimeout, nil, "uninstall")
	writeCommandResult(w, out, err)
}

func (s *Skill) handleRestart(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "restart")
	writeCommandResult(w, out, err)
}

func (s *Skill) handleReloadLFD(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "reload-lfd")
	writeCommandResult(w, out, err)
}

func (s *Skill) handleListAllow(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "list-allow")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": parseIPList(out)})
}

func (s *Skill) handleListDeny(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "list-deny")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": parseIPList(out)})
}

func (s *Skill) handleListTempbans(w http.ResponseWriter, r *http.Request) {
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "list-tempbans")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"raw": out})
}

func (s *Skill) handleAllowIP(w http.ResponseWriter, r *http.Request) {
	ip, comment, err := decodeIPBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "allow-ip", ip, comment)
	writeCommandResult(w, out, err)
}

func (s *Skill) handleDenyIP(w http.ResponseWriter, r *http.Request) {
	ip, comment, err := decodeIPBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "deny-ip", ip, comment)
	writeCommandResult(w, out, err)
}

func (s *Skill) handleRemoveIP(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	if net.ParseIP(strings.SplitN(ip, "/", 2)[0]) == nil {
		writeError(w, http.StatusBadRequest, "invalid IP")
		return
	}
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "remove-ip", ip)
	writeCommandResult(w, out, err)
}

func (s *Skill) handleReadConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := allowedConfigs[name]; !ok {
		writeError(w, http.StatusBadRequest, "config name not allowed")
		return
	}
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "read-config", name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "content": out})
}

func (s *Skill) handleWriteConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := allowedConfigs[name]; !ok {
		writeError(w, http.StatusBadRequest, "config name not allowed")
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	content := strings.ReplaceAll(body.Content, "\x00", "")
	if len(content) > 512*1024 {
		writeError(w, http.StatusRequestEntityTooLarge, "config too large")
		return
	}
	out, err := s.runHelper(r.Context(), commandTimeout, strings.NewReader(content), "write-config", name)
	writeCommandResult(w, out, err)
}

func (s *Skill) handleTailLog(w http.ResponseWriter, r *http.Request) {
	lines := 200
	if q := r.URL.Query().Get("lines"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 5000 {
			lines = n
		}
	}
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "tail-log", strconv.Itoa(lines))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines, "content": out})
}

func (s *Skill) getClientIP(w http.ResponseWriter, r *http.Request) {
	// Available to any signed-in user so the manual "Add my IP" button in
	// the Firewall panel can show the value that would be sent, and so
	// non-admins can at least see it before asking an admin to allow it.
	writeJSON(w, http.StatusOK, map[string]string{"ip": clientIP(r)})
}

func (s *Skill) handleAllowMyIP(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.CurrentUser(r)
	ip := clientIP(r)
	comment := fmt.Sprintf("Stack Manager admin %s manual", user.Username)
	out, err := s.runHelper(r.Context(), commandTimeout, nil, "allow-ip", ip, sanitizeComment(comment))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ip": ip, "output": out})
}

// --- csf.conf structured settings -------------------------------------------

type confSettings struct {
	Testing       string `json:"testing"`
	TCPIn         string `json:"tcp_in"`
	TCPOut        string `json:"tcp_out"`
	UDPIn         string `json:"udp_in"`
	UDPOut        string `json:"udp_out"`
	TCP6In        string `json:"tcp6_in"`
	TCP6Out       string `json:"tcp6_out"`
	UDP6In        string `json:"udp6_in"`
	UDP6Out       string `json:"udp6_out"`
	Syslog        string `json:"restrict_syslog"`
	SynFlood      string `json:"synflood"`
	ConnLimit     string `json:"connlimit"`
	PortFlood     string `json:"portflood"`
	Docker        string `json:"docker"`
}

var confKeys = []string{
	"TESTING", "TCP_IN", "TCP_OUT", "UDP_IN", "UDP_OUT",
	"TCP6_IN", "TCP6_OUT", "UDP6_IN", "UDP6_OUT",
	"RESTRICT_SYSLOG", "SYNFLOOD", "CONNLIMIT", "PORTFLOOD", "DOCKER",
}

func (s *Skill) handleGetConfSettings(w http.ResponseWriter, r *http.Request) {
	raw, err := s.runHelper(r.Context(), commandTimeout, nil, "read-config", "csf.conf")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	parsed := parseConfKeys(raw, confKeys)
	settings := confSettings{
		Testing:   parsed["TESTING"],
		TCPIn:     parsed["TCP_IN"],
		TCPOut:    parsed["TCP_OUT"],
		UDPIn:     parsed["UDP_IN"],
		UDPOut:    parsed["UDP_OUT"],
		TCP6In:    parsed["TCP6_IN"],
		TCP6Out:   parsed["TCP6_OUT"],
		UDP6In:    parsed["UDP6_IN"],
		UDP6Out:   parsed["UDP6_OUT"],
		Syslog:    parsed["RESTRICT_SYSLOG"],
		SynFlood:  parsed["SYNFLOOD"],
		ConnLimit: parsed["CONNLIMIT"],
		PortFlood: parsed["PORTFLOOD"],
		Docker:    parsed["DOCKER"],
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Skill) handleSaveConfSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	raw, err := s.runHelper(r.Context(), commandTimeout, nil, "read-config", "csf.conf")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated := strings.ReplaceAll(applyConfChanges(raw, body), "\x00", "")
	_, err = s.runHelper(r.Context(), commandTimeout, strings.NewReader(updated), "write-config", "csf.conf")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": "csf.conf", "hint": "Click Restart csf to apply."})
}

func parseConfKeys(content string, keys []string) map[string]string {
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	result := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if !keySet[key] {
			continue
		}
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		result[key] = val
	}
	return result
}

func applyConfChanges(content string, changes map[string]string) string {
	keyMap := map[string]string{
		"testing": "TESTING", "tcp_in": "TCP_IN", "tcp_out": "TCP_OUT",
		"udp_in": "UDP_IN", "udp_out": "UDP_OUT",
		"tcp6_in": "TCP6_IN", "tcp6_out": "TCP6_OUT",
		"udp6_in": "UDP6_IN", "udp6_out": "UDP6_OUT",
		"restrict_syslog": "RESTRICT_SYSLOG", "synflood": "SYNFLOOD",
		"connlimit": "CONNLIMIT", "portflood": "PORTFLOOD", "docker": "DOCKER",
	}
	updates := map[string]string{}
	for jsonKey, val := range changes {
		if confKey, ok := keyMap[jsonKey]; ok {
			updates[confKey] = val
		}
	}
	if len(updates) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		if newVal, ok := updates[key]; ok {
			lines[i] = key + " = \"" + newVal + "\""
			delete(updates, key)
		}
	}
	return strings.Join(lines, "\n")
}

// --- helpers -----------------------------------------------------------------

func decodeIPBody(r *http.Request) (string, string, error) {
	var body struct {
		IP      string `json:"ip"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", "", errors.New("invalid request body")
	}
	ip := strings.TrimSpace(body.IP)
	if net.ParseIP(strings.SplitN(ip, "/", 2)[0]) == nil {
		return "", "", errors.New("invalid IP")
	}
	comment := sanitizeComment(body.Comment)
	if comment == "" {
		return "", "", errors.New("comment is required")
	}
	return ip, comment, nil
}

func (s *Skill) runHelper(ctx context.Context, timeout time.Duration, stdin io.Reader, sub string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Spawn a throwaway privileged container with --network=host and
	// chroot into the host root to run the helper. The --network=host
	// flag means the container uses the host's network stack directly
	// and survives iptables flushes (csf -r). The csfpost.sh Docker
	// restart uses a background nohup+sleep so it fires AFTER this
	// helper container returns its output.
	dockerArgs := []string{"run", "--rm"}
	if stdin != nil {
		dockerArgs = append(dockerArgs, "-i")
	}
	dockerArgs = append(dockerArgs,
		"--privileged",
		"--network=host",
		"--pid=host",
		"-v", "/:/host",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		s.baseImagePrefix+defaultHelperImage,
		"chroot", "/host",
		s.helperPath, sub,
	)
	dockerArgs = append(dockerArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...) //nolint:gosec // helper path is a hardcoded default; sub+args validated by the helper
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		combined := stderr.String() + " " + out
		// nsenter / chroot returns 127 when the target binary is
		// missing. Detect the "helper not present" case so callers
		// can show an install prompt instead of an error toast.
		if strings.Contains(combined, s.helperPath) &&
			(strings.Contains(combined, "No such file") || strings.Contains(combined, "can't execute") || strings.Contains(combined, "not found")) {
			return out, errHelperMissing
		}
		return out, fmt.Errorf("helper failed: %s: %s", err.Error(), strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func parseStatus(statusOut, versionOut string) StatusResult {
	result := StatusResult{Raw: statusOut}
	for _, line := range strings.Split(statusOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "installed":
			result.Installed = value == "true"
		case "testing_mode":
			result.TestingMode = value
		case "iptables_rules":
			if n, err := strconv.Atoi(value); err == nil {
				result.IptablesRules = n
			}
		case "lfd_active":
			result.LFDActive = value == "true"
		}
	}
	result.Version = strings.TrimSpace(strings.SplitN(versionOut, "\n", 2)[0])
	return result
}

type IPEntry struct {
	IP      string `json:"ip"`
	Comment string `json:"comment"`
	Raw     string `json:"raw"`
}

func parseIPList(content string) []IPEntry {
	entries := make([]IPEntry, 0)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		entry := IPEntry{Raw: trimmed}
		// CSF allow/deny lines are `<ip> # comment` or just `<ip>`.
		if idx := strings.Index(trimmed, "#"); idx >= 0 {
			entry.IP = strings.TrimSpace(trimmed[:idx])
			entry.Comment = strings.TrimSpace(trimmed[idx+1:])
		} else {
			entry.IP = trimmed
		}
		entries = append(entries, entry)
	}
	return entries
}

func sanitizeComment(comment string) string {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return ""
	}
	// Match the helper's COMMENT_RE: [A-Za-z0-9 ._@:/-]{1,120}
	var b strings.Builder
	for _, r := range comment {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '.' || r == '_' || r == '@' || r == ':' || r == '/' || r == '-':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

func clientIP(r *http.Request) string {
	// The built-in nginx sets X-Real-IP to the browser's address. Read
	// it directly rather than relying on chi.RealIP middleware (which
	// only trusts loopback as a proxy source by default, and Docker
	// bridge IPs like 172.18.0.x don't qualify).
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// --- response helpers --------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "error",
		"error":     msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func writeCommandResult(w http.ResponseWriter, out string, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error()+": "+out)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}
