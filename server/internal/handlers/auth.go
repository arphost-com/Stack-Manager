package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	cmauth "github.com/arphost-com/Stack-Manager/server/internal/auth"
	"github.com/arphost-com/Stack-Manager/server/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/pquerna/otp"
)

// totpQRDataURI renders an otpauth:// URL to a base64 PNG data URI so the
// enrollment QR is generated on the server. This keeps the TOTP secret (which
// is embedded in otpauth URLs) inside our own trust boundary instead of
// shipping it to a third-party QR image service, and works on airgapped hosts.
func totpQRDataURI(otpURL string) string {
	key, err := otp.NewKeyFromURL(otpURL)
	if err != nil {
		return ""
	}
	img, err := key.Image(220, 220)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// IPAllower is implemented by the firewall skill. Kept as an interface so
// auth doesn't depend on skills package.
type IPAllower interface {
	AllowIP(ctx context.Context, ip, comment string) error
}

type AuthHandler struct {
	Store    *cmauth.Store
	Sessions *cmauth.SessionManager
	// IPAllower is optional. When set, successful logins fire off a
	// best-effort call to allowlist the caller's IP in CSF. Errors are
	// logged and swallowed so a firewall problem cannot break login.
	IPAllower IPAllower
}

func NewAuthHandler(store *cmauth.Store, sessions *cmauth.SessionManager) *AuthHandler {
	return &AuthHandler{Store: store, Sessions: sessions}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, ok := h.Store.Authenticate(body.Username, body.Password)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	// If the user has TOTP enabled, don't issue a session yet — return
	// totp_required so the frontend shows a code input. A short-lived
	// "totp_pending" token lets the second POST (/auth/totp/login)
	// finish the flow without re-sending the password.
	if user.TOTPEnabled {
		pending, err := h.Sessions.CreatePending(user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"totp_required": true,
			"totp_token":    pending.Token,
			"user":          user,
		})
		return
	}

	session, err := h.Sessions.Create(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.allowlistLoginIP(r, user.Username)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      session.Token,
		"expires_at": session.ExpiresAt.Format(time.RFC3339),
		"user":       user,
	})
}

// TOTPLogin completes the second step of the 2FA login flow.
func (h *AuthHandler) TOTPLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TOTPToken string `json:"totp_token"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pending, ok := h.Sessions.GetPending(body.TOTPToken)
	if !ok {
		writeError(w, http.StatusUnauthorized, "totp token expired or invalid — re-enter your password")
		return
	}
	h.Sessions.DeletePending(body.TOTPToken)
	if !h.Store.TOTPValidateCode(pending.User.Username, strings.TrimSpace(body.Code)) {
		writeError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}
	session, err := h.Sessions.Create(pending.User)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.allowlistLoginIP(r, pending.User.Username)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      session.Token,
		"expires_at": session.ExpiresAt.Format(time.RFC3339),
		"user":       pending.User,
	})
}

// TOTPEnroll generates a new TOTP secret for the calling user.
func (h *AuthHandler) TOTPEnroll(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	secret, backupCodes, otpURL, err := h.Store.TOTPEnroll(user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"secret":       secret,
		"backup_codes": backupCodes,
		"otp_url":      otpURL,
		"qr_data_uri":  totpQRDataURI(otpURL),
	})
}

// TOTPVerify checks a code and enables TOTP if correct.
func (h *AuthHandler) TOTPVerify(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.Store.TOTPVerifyAndEnable(user.Username, strings.TrimSpace(body.Code)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"totp_enabled": true})
}

// TOTPDisable removes TOTP from the calling user's account.
func (h *AuthHandler) TOTPDisable(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, valid := h.Store.Authenticate(user.Username, body.Password); !valid {
		writeError(w, http.StatusUnauthorized, "password incorrect")
		return
	}
	if err := h.Store.TOTPDisable(user.Username); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"totp_enabled": false})
}

// TOTPResetUser lets an admin remove TOTP from another user's account.
func (h *AuthHandler) TOTPResetUser(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	username := chi.URLParam(r, "username")
	if err := h.Store.TOTPDisable(username); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username, "totp": "disabled"})
}

// allowlistLoginIP fires off a best-effort CSF allow for the caller. It
// uses a fresh background context (not r.Context()) so the auth response
// can return before the helper finishes; a slow sudo/csf must not make
// login look slow to the browser.
func (h *AuthHandler) allowlistLoginIP(r *http.Request, username string) {
	if h.IPAllower == nil {
		return
	}
	ip := loginClientIP(r)
	if ip == "" {
		return
	}
	comment := "Stack Manager login " + strings.TrimSpace(username)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.IPAllower.AllowIP(ctx, ip, comment); err != nil {
			log.Printf("firewall: allowlist %s failed: %v", ip, err)
		}
	}()
}

func loginClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := cmauth.BearerToken(r)
	if token != "" {
		h.Sessions.Delete(token)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.Store.ListUsers())
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.Store.CreateUser(body.Username, body.Password, body.Role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, _ := h.Store.GetUser(body.Username)
	writeJSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	username := chi.URLParam(r, "username")
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.Store.SetPassword(username, body.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

// ChangeOwnPassword lets any authenticated user rotate their own password by
// providing the current one. This is the "My Account" flow - separate from
// the admin-only SetPassword above which is for resetting someone else's
// login. The middleware already required auth, so the caller identity is
// trusted.
func (h *AuthHandler) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.CurrentUser(r)
	if !ok || user.Username == "" {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.Store.ChangeOwnPassword(user.Username, body.CurrentPassword, body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": user.Username})
}

func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !middleware.RequireAdmin(w, r) {
		return
	}
	username := chi.URLParam(r, "username")
	if err := h.Store.DeleteUser(username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": username})
}
