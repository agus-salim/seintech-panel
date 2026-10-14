package middleware

import (
	"context"
	"net/http"

	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/model"
)

type contextKey string

const (
	SessionContextKey contextKey = "session"
)

// SecurityHeaders sets standard hardened HTTP response headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// RequireAuth validates the session cookie and attaches session context.
func RequireAuth(authMgr *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("seintech_session")
			if err != nil || cookie.Value == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			sess, err := authMgr.ValidateSession(cookie.Value)
			if err != nil {
				http.SetCookie(w, &http.Cookie{
					Name:   "seintech_session",
					Value:  "",
					Path:   "/",
					MaxAge: -1,
				})
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), SessionContextKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole enforces specific RBAC roles (e.g. Admin only for settings).
func RequireRole(allowedRoles ...model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := r.Context().Value(SessionContextKey).(*auth.Session)
			if !ok || sess == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			permitted := false
			for _, role := range allowedRoles {
				if sess.Role == role {
					permitted = true
					break
				}
			}

			if !permitted {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
