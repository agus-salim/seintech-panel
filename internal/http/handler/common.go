package handler

import (
	"net/http"
	"strings"

	"github.com/seintech/panel/internal/core/auth"
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
