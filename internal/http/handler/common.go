package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/core/i18n"
	"github.com/seintech/panel/internal/http/middleware"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
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
	Lang            string
}

func (b BasePageData) GetLang() string {
	if b.Lang != "" {
		return b.Lang
	}
	return "en"
}

func (b BasePageData) T(phrase string, args ...any) string {
	return i18n.T(b.GetLang(), phrase, args...)
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

func NewBaseData(r *http.Request, title, pageTitle, activeNav, hostname string, defaultLang ...string) BasePageData {
	sess := GetSession(r)
	initial := "A"
	if len(sess.Username) > 0 {
		initial = strings.ToUpper(string(sess.Username[0]))
	}

	if hostname == "" {
		if sys, err := os.Hostname(); err == nil && sys != "" {
			hostname = sys
		} else {
			hostname = "localhost"
		}
	}

	lang := "en"
	if len(defaultLang) > 0 && defaultLang[0] != "" {
		lang = defaultLang[0]
	}
	lang = i18n.GetLangFromRequest(r, lang)

	return BasePageData{
		Title:           i18n.T(lang, title),
		PageTitle:       i18n.T(lang, pageTitle),
		ActiveNav:       activeNav,
		Hostname:        hostname,
		Username:        sess.Username,
		UsernameInitial: initial,
		UserRole:        string(sess.Role),
		Lang:            lang,
	}
}

// GetAccessibleUserIDs returns a map of user IDs that the current session is authorized to view or manage.
// Returns nil if session is Admin (meaning all users are accessible).
func GetAccessibleUserIDs(sess *auth.Session, repo *storage.Repository) map[string]bool {
	if sess == nil || sess.Role == model.RoleAdmin {
		return nil
	}

	allowed := make(map[string]bool)
	allowed[sess.UserID] = true

	if sess.Role == model.RoleReseller {
		allUsers, _ := repo.GetUsers()
		for _, u := range allUsers {
			if u.ResellerID == sess.UserID {
				allowed[u.ID] = true
			}
		}
	}

	return allowed
}

// IsUserAccessible verifies if the session has permission to access resources belonging to targetUserID.
func IsUserAccessible(sess *auth.Session, repo *storage.Repository, targetUserID string) bool {
	if sess == nil || sess.Role == model.RoleAdmin {
		return true
	}
	if targetUserID == "" {
		return false
	}
	allowed := GetAccessibleUserIDs(sess, repo)
	return allowed[targetUserID]
}
