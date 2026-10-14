package handler

import (
	"net/http"

	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/web"
)

type AuthHandler struct {
	authMgr *auth.AuthManager
}

func NewAuthHandler(authMgr *auth.AuthManager) *AuthHandler {
	return &AuthHandler{authMgr: authMgr}
}

type LoginPageData struct {
	Error string
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to home
	if cookie, err := r.Cookie("seintech_session"); err == nil && cookie.Value != "" {
		if _, err := h.authMgr.ValidateSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	_ = web.RenderSingleTemplate(w, "login.html", LoginPageData{})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	sess, err := h.authMgr.Authenticate(username, password, 120) // 120 minutes timeout
	if err != nil {
		_ = web.RenderSingleTemplate(w, "login.html", LoginPageData{Error: "Invalid username or password"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "seintech_session",
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   120 * 60,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("seintech_session"); err == nil {
		h.authMgr.InvalidateSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "seintech_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
