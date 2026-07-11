package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	cmauth "github.com/arphost-com/Stack-Manager/server/internal/auth"
)

type contextKey string

const UserContextKey contextKey = "stack-manager-user"

// RequireAPIKey validates the X-API-Key header against the current key.
func RequireAPIKey(keyFn func() string) func(http.Handler) http.Handler {
	return RequireAuth(keyFn, nil)
}

// RequireAuth validates either a bearer session token or the legacy X-API-Key.
// keyFn returns the current API key (stored in the DB, so a roll takes effect
// without a restart).
// EventSource in the browser can not set custom request headers, so this
// middleware also honours ?token= and ?api_key= query parameters as a
// fallback specifically for streaming endpoints. Query auth is still bound
// to HTTPS, so it is no weaker than the header equivalents.
func RequireAuth(keyFn func() string, sessions *cmauth.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sessions != nil {
				token := cmauth.BearerToken(r)
				if token == "" {
					token = strings.TrimSpace(r.URL.Query().Get("token"))
				}
				if token != "" {
					session, ok := sessions.Get(token)
					if !ok {
						writeAuthError(w, "invalid or expired session")
						return
					}
					ctx := context.WithValue(r.Context(), UserContextKey, session.User)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			provided := r.Header.Get("X-API-Key")
			if provided == "" {
				provided = strings.TrimSpace(r.URL.Query().Get("api_key"))
			}
			if provided == "" {
				writeAuthError(w, "missing credentials")
				return
			}

			apiKey := ""
			if keyFn != nil {
				apiKey = keyFn()
			}
			if apiKey == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				writeAuthError(w, "invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, cmauth.PublicUser{Username: "api-key", Role: cmauth.RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CurrentUser(r *http.Request) (cmauth.PublicUser, bool) {
	user, ok := r.Context().Value(UserContextKey).(cmauth.PublicUser)
	return user, ok
}

func RequireAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, ok := CurrentUser(r)
	if !ok || user.Role != cmauth.RoleAdmin {
		writeAuthErrorStatus(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

func writeAuthError(w http.ResponseWriter, msg string) {
	writeAuthErrorStatus(w, http.StatusUnauthorized, msg)
}

func writeAuthErrorStatus(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "error",
		"error":     msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
