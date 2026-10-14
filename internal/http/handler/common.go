package handler

import (
	"net/http"
	"strings"

	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/http/middleware"
	"github.com/seintech/panel/internal/model"
)

type BasePageData struct {
	Title           string
	PageTitle       string
	ActiveNav       string
	Hostname        string
	Username        string
	UsernameInitial string
	UserRole        string
	FlashMessage    string
	FlashType       string
}

func GetSession(r *http.Request) *auth.Session {
	sess, ok := r.Context().Value(middleware.SessionContextKey).(*auth.Session)
	if !ok || sess == nil {
		return &auth.Session{
			Username: "Admin",
			Role:     model.RoleAdmin,
		}
	}
	return sess
}

func NewBaseData(r *http.Request, title, pageTitle, activeNav, hostname string) BasePageData {
	sess := GetSession(r)
	initial := "A"
	if len(sess.Username) > 0 {
		initial = strings.ToUpper(string(sess.Username[0]))
	}

	return BasePageData{
		Title:           title,
		PageTitle:       pageTitle,
		ActiveNav:       activeNav,
		Hostname:        hostname,
		Username:        sess.Username,
		UsernameInitial: initial,
		UserRole:        string(sess.Role),
	}
}
