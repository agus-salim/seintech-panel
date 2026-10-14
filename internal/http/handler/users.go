package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"time"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type UserHandler struct {
	repo *storage.Repository
	cfg  *config.Config
}

func NewUserHandler(repo *storage.Repository, cfg *config.Config) *UserHandler {
	return &UserHandler{repo: repo, cfg: cfg}
}

type UsersPageData struct {
	BasePageData
	Users []model.User
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	users, _ := h.repo.GetUsers()

	data := UsersPageData{
		BasePageData: NewBaseData(r, "Users & Quotas", "User, Reseller & RBAC Administration", "users", settings.Hostname),
		Users:        users,
	}

	_ = web.RenderTemplate(w, "users.html", data)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")
	role := model.Role(r.FormValue("role"))

	if username != "" && password != "" {
		hash, _ := auth.HashPassword(password)
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)

		user := model.User{
			ID:           id,
			Username:     username,
			PasswordHash: hash,
			Email:        email,
			Role:         role,
			HomeDir:      filepath.Join(h.cfg.WebRoot, username),
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		_ = h.repo.UpsertUser(user)
	}

	http.Redirect(w, r, "/users?success=created", http.StatusSeeOther)
}
