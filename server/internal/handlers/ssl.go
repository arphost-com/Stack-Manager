package handlers

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SSLHandler manages the TLS material used by the web container. Cert files
// live under STATE_DIR/ssl/ so they persist across container rebuilds and are
// shared with nginx via a bind mount.
type SSLHandler struct {
	SSLDir          string
	BaseImagePrefix string
	OwnContainer    string
	CertbotImage    string
}

func NewSSLHandler(stateDir, baseImagePrefix string) *SSLHandler {
	host, _ := os.Hostname()
	certbot := strings.TrimSpace(os.Getenv("CERTBOT_IMAGE"))
	if certbot == "" {
		certbot = "certbot/certbot:latest"
	}
	return &SSLHandler{
		SSLDir:          filepath.Join(stateDir, "ssl"),
		BaseImagePrefix: baseImagePrefix,
		OwnContainer:    strings.TrimSpace(host),
		CertbotImage:    certbot,
	}
}

type sslCertInfo struct {
	Mode              string    `json:"mode"`
	Present           bool      `json:"present"`
	CN                string    `json:"cn"`
	Issuer            string    `json:"issuer"`
	SANs              []string  `json:"sans"`
	NotBefore         time.Time `json:"not_before,omitempty"`
	NotAfter          time.Time `json:"not_after,omitempty"`
	DaysLeft          int       `json:"days_left"`
	SelfSigned        bool      `json:"self_signed"`
	Domain            string    `json:"domain,omitempty"`
	HTTPPort          int       `json:"http_port"`
	SSLPort           int       `json:"ssl_port"`
	LetsEncryptReady  bool      `json:"letsencrypt_ready"`
	LetsEncryptReason string    `json:"letsencrypt_reason,omitempty"`
}

// Get returns metadata about the currently installed certificate.
func (h *SSLHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.certInfo())
}

type sslSelfSignedRequest struct {
	CN        string   `json:"cn"`
	ExtraSANs []string `json:"extra_sans"`
	Days      int      `json:"days"`
}

// RegenerateSelfSigned rebuilds the self-signed cert in place and reloads nginx.
func (h *SSLHandler) RegenerateSelfSigned(w http.ResponseWriter, r *http.Request) {
	var req sslSelfSignedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Days <= 0 || req.Days > 3650 {
		req.Days = 3650
	}
	cn := strings.TrimSpace(req.CN)
	if cn == "" {
		cn = "stack-manager"
	}
	if !isValidHostname(cn) {
		writeError(w, http.StatusBadRequest, "cn must be a valid hostname")
		return
	}
	sans := []string{"DNS:" + cn, "DNS:localhost", "IP:127.0.0.1"}
	for _, s := range req.ExtraSANs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if isValidIP(s) {
			sans = append(sans, "IP:"+s)
			continue
		}
		if isValidHostname(s) {
			sans = append(sans, "DNS:"+s)
			continue
		}
		writeError(w, http.StatusBadRequest, "invalid SAN value: "+s)
		return
	}

	script := fmt.Sprintf(`set -eu
apk add --no-cache openssl >/dev/null 2>&1
umask 077
openssl req -x509 -nodes -newkey rsa:2048 \
  -days %d \
  -subj "/CN=%s" \
  -addext "subjectAltName=%s" \
  -addext "keyUsage=digitalSignature,keyEncipherment" \
  -addext "extendedKeyUsage=serverAuth" \
  -keyout /state/ssl/privkey.pem \
  -out /state/ssl/fullchain.pem
printf 'self-signed\n' > /state/ssl/mode
rm -f /state/ssl/domain
chmod 644 /state/ssl/fullchain.pem /state/ssl/mode
chmod 600 /state/ssl/privkey.pem
`, req.Days, cn, strings.Join(sans, ","))

	output, err := h.runAlpineHelper(script)
	if err != nil {
		writeError(w, http.StatusBadGateway, "regenerate failed: "+err.Error()+" - "+strings.TrimSpace(string(output)))
		return
	}
	reloadErr := h.reloadNginx()
	resp := map[string]interface{}{"cert": h.certInfo()}
	if reloadErr != nil {
		resp["reload_warning"] = reloadErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

type sslLetsEncryptRequest struct {
	Domain  string `json:"domain"`
	Email   string `json:"email"`
	Staging bool   `json:"staging"`
}

// EnableLetsEncrypt runs certbot in a helper container to obtain a cert via
// HTTP-01, then copies the issued cert into the nginx-visible paths.
func (h *SSLHandler) EnableLetsEncrypt(w http.ResponseWriter, r *http.Request) {
	var req sslLetsEncryptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	email := strings.TrimSpace(req.Email)
	if !isValidHostname(domain) {
		writeError(w, http.StatusBadRequest, "domain must be a valid DNS name")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		writeError(w, http.StatusBadRequest, "email is required and must be valid")
		return
	}
	if httpPort := currentHTTPPort(); httpPort != 80 {
		writeError(w, http.StatusPreconditionFailed, fmt.Sprintf("Let's Encrypt HTTP-01 requires WEB_HTTP_PORT=80 (currently %d). Update .env and redeploy first.", httpPort))
		return
	}

	args := []string{
		"run", "--rm", "--user", "0:0",
		"--volumes-from", h.OwnContainer,
		h.CertbotImage,
		"certonly", "--webroot",
		"-w", "/state/ssl/acme-webroot",
		"-d", domain,
		"--email", email,
		"--agree-tos", "--no-eff-email", "--non-interactive",
		"--preferred-challenges", "http",
		"--config-dir", "/state/ssl/letsencrypt",
		"--work-dir", "/state/ssl/letsencrypt/work",
		"--logs-dir", "/state/ssl/letsencrypt/logs",
	}
	if req.Staging {
		args = append(args, "--staging")
	}
	output, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		writeError(w, http.StatusBadGateway, "certbot failed: "+err.Error()+"\n"+strings.TrimSpace(string(output)))
		return
	}

	if err := h.installLetsEncryptCert(domain); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	reloadErr := h.reloadNginx()
	resp := map[string]interface{}{
		"cert":           h.certInfo(),
		"certbot_output": string(output),
	}
	if reloadErr != nil {
		resp["reload_warning"] = reloadErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// RenewLetsEncrypt attempts to renew any Let's Encrypt cert whose expiry is
// within the certbot default renewal window.
func (h *SSLHandler) RenewLetsEncrypt(w http.ResponseWriter, r *http.Request) {
	if !h.hasLetsEncryptState() {
		writeError(w, http.StatusPreconditionFailed, "no Let's Encrypt state to renew")
		return
	}
	if httpPort := currentHTTPPort(); httpPort != 80 {
		writeError(w, http.StatusPreconditionFailed, fmt.Sprintf("Let's Encrypt HTTP-01 renewal requires WEB_HTTP_PORT=80 (currently %d).", httpPort))
		return
	}
	args := []string{
		"run", "--rm", "--user", "0:0",
		"--volumes-from", h.OwnContainer,
		h.CertbotImage,
		"renew",
		"--webroot", "-w", "/state/ssl/acme-webroot",
		"--config-dir", "/state/ssl/letsencrypt",
		"--work-dir", "/state/ssl/letsencrypt/work",
		"--logs-dir", "/state/ssl/letsencrypt/logs",
		"--non-interactive",
	}
	output, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		writeError(w, http.StatusBadGateway, "certbot renew failed: "+err.Error()+"\n"+strings.TrimSpace(string(output)))
		return
	}
	domain, _ := h.readMeta("domain")
	if domain != "" {
		if err := h.installLetsEncryptCert(domain); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	reloadErr := h.reloadNginx()
	resp := map[string]interface{}{
		"cert":           h.certInfo(),
		"certbot_output": string(output),
	}
	if reloadErr != nil {
		resp["reload_warning"] = reloadErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- internals ---

func (h *SSLHandler) certInfo() sslCertInfo {
	info := sslCertInfo{Mode: "unknown"}
	if mode, _ := h.readMeta("mode"); mode != "" {
		info.Mode = mode
	}
	if domain, _ := h.readMeta("domain"); domain != "" {
		info.Domain = domain
	}
	info.HTTPPort = currentHTTPPort()
	info.SSLPort = currentSSLPort()
	if info.HTTPPort == 80 && info.SSLPort == 443 {
		info.LetsEncryptReady = true
	} else {
		info.LetsEncryptReady = false
		info.LetsEncryptReason = fmt.Sprintf("set WEB_HTTP_PORT=80 and WEB_SSL_PORT=443 in .env (currently %d/%d)", info.HTTPPort, info.SSLPort)
	}

	certBytes, err := os.ReadFile(filepath.Join(h.SSLDir, "fullchain.pem"))
	if err != nil {
		return info
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return info
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return info
	}
	info.Present = true
	info.CN = cert.Subject.CommonName
	info.Issuer = cert.Issuer.CommonName
	sans := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}
	info.SANs = sans
	info.NotBefore = cert.NotBefore
	info.NotAfter = cert.NotAfter
	info.DaysLeft = int(time.Until(cert.NotAfter).Hours() / 24)
	info.SelfSigned = bytes.Equal(cert.RawSubject, cert.RawIssuer)
	return info
}

func (h *SSLHandler) readMeta(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(h.SSLDir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (h *SSLHandler) hasLetsEncryptState() bool {
	if mode, _ := h.readMeta("mode"); mode == "letsencrypt" {
		return true
	}
	if _, err := os.Stat(filepath.Join(h.SSLDir, "letsencrypt", "live")); err == nil {
		return true
	}
	return false
}

// installLetsEncryptCert copies the freshly issued cert into the paths nginx
// serves from and updates the mode/domain markers.
func (h *SSLHandler) installLetsEncryptCert(domain string) error {
	liveDir := filepath.Join(h.SSLDir, "letsencrypt", "live", domain)
	if err := copyFile(filepath.Join(liveDir, "fullchain.pem"), filepath.Join(h.SSLDir, "fullchain.pem"), 0o644); err != nil {
		return fmt.Errorf("copy fullchain: %w", err)
	}
	if err := copyFile(filepath.Join(liveDir, "privkey.pem"), filepath.Join(h.SSLDir, "privkey.pem"), 0o600); err != nil {
		return fmt.Errorf("copy privkey: %w", err)
	}
	if err := os.WriteFile(filepath.Join(h.SSLDir, "mode"), []byte("letsencrypt\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(h.SSLDir, "domain"), []byte(domain+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

func (h *SSLHandler) runAlpineHelper(script string) ([]byte, error) {
	if h.OwnContainer == "" {
		return nil, fmt.Errorf("own container hostname unavailable")
	}
	image := h.BaseImagePrefix + "alpine:3.22"
	args := []string{
		"run", "--rm", "--user", "0:0",
		"--volumes-from", h.OwnContainer,
		image, "sh", "-c", script,
	}
	return exec.Command("docker", args...).CombinedOutput()
}

func (h *SSLHandler) reloadNginx() error {
	project, err := h.composeProject()
	if err != nil {
		return err
	}
	out, err := exec.Command("docker", "ps", "-q",
		"--filter", "label=com.docker.compose.service=web",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "status=running",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("find web container: %v - %s", err, strings.TrimSpace(string(out)))
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return fmt.Errorf("web container not running")
	}
	// SIGHUP triggers nginx to reload its config and re-read the certs
	// without dropping open connections.
	if out, err := exec.Command("docker", "kill", "--signal=HUP", id).CombinedOutput(); err != nil {
		return fmt.Errorf("reload nginx: %v - %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (h *SSLHandler) composeProject() (string, error) {
	if h.OwnContainer == "" {
		return "", fmt.Errorf("own container hostname unavailable")
	}
	out, err := exec.Command("docker", "inspect", "--format",
		"{{index .Config.Labels \"com.docker.compose.project\"}}",
		h.OwnContainer).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("inspect self: %v - %s", err, strings.TrimSpace(string(out)))
	}
	label := strings.TrimSpace(string(out))
	if label == "" {
		return "", fmt.Errorf("compose project label missing on %s", h.OwnContainer)
	}
	return label, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func currentHTTPPort() int {
	if v := os.Getenv("WEB_HTTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if v := os.Getenv("WEB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8193
}

func currentSSLPort() int {
	if v := os.Getenv("WEB_SSL_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8993
}

var (
	hostnameRe = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)
	ipv4Re     = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
)

func isValidHostname(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	return hostnameRe.MatchString(s)
}

func isValidIP(s string) bool {
	return ipv4Re.MatchString(s)
}
