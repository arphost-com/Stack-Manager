package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	cmauth "github.com/arphost-com/Compose-Manager/server/internal/auth"
	"github.com/arphost-com/Compose-Manager/server/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type AuthHandler struct {
	Store    *cmauth.Store
	Sessions *cmauth.SessionManager
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

	session, err := h.Sessions.Create(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      session.Token,
		"expires_at": session.ExpiresAt.Format(time.RFC3339),
		"user":       user,
	})
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
