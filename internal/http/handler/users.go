package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/core/system"
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

type UserViewItem struct {
	model.User
	Quota        model.Quota
	DomainCount  int
	DBCount      int
	CanDelete    bool
	CanEditQuota bool
	ResellerName string
}

type UsersPageData struct {
	BasePageData
	Users      []UserViewItem
	IsReseller bool
	IsAdmin    bool
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess.Role == model.RoleUser {
		http.Error(w, "Akses ditolak: End-User tidak memiliki izin untuk mengelola pengguna", http.StatusForbidden)
		return
	}

	settings, _ := h.repo.GetSettings()
	allUsers, _ := h.repo.GetUsers()
	allDomains, _ := h.repo.GetDomains()
	allDBs, _ := h.repo.GetDatabases()

	// Map username/id to domain list and database list
	userDomainMap := make(map[string][]model.Domain)
	for _, d := range allDomains {
		userDomainMap[d.UserID] = append(userDomainMap[d.UserID], d)
	}

	userDBMap := make(map[string][]model.Database)
	for _, db := range allDBs {
		userDBMap[db.UserID] = append(userDBMap[db.UserID], db)
	}

	userLookup := make(map[string]model.User)
	for _, u := range allUsers {
		userLookup[u.ID] = u
	}

	accessibleIDs := GetAccessibleUserIDs(sess, h.repo)

	var items []UserViewItem
	for _, u := range allUsers {
		// Filter visibility
		if accessibleIDs != nil && !accessibleIDs[u.ID] {
			continue
		}

		q, _ := h.repo.GetQuota(u.ID)
		if q == nil {
			q = &model.Quota{UserID: u.ID}
		}

		uDomains := userDomainMap[u.ID]
		uDBs := userDBMap[u.ID]

		// Realtime calculations of used disk and ram
		q.UsedDiskMB = system.CalculateUserDiskUsage(h.cfg.WebRoot, u, uDomains)
		q.UsedRamMB = system.CalculateUserRamUsage(r.Context(), u, uDomains)
		q.UsedDomains = len(uDomains)
		q.UsedDatabases = len(uDBs)

		resellerName := ""
		if u.ResellerID != "" {
			if rUser, ok := userLookup[u.ResellerID]; ok {
				resellerName = rUser.Username
			}
		}

		canDelete := u.ID != sess.UserID && (sess.Role == model.RoleAdmin || u.ResellerID == sess.UserID)
		canEditQuota := sess.Role == model.RoleAdmin || u.ResellerID == sess.UserID || (sess.Role == model.RoleReseller && u.ID == sess.UserID)

		items = append(items, UserViewItem{
			User:         u,
			Quota:        *q,
			DomainCount:  len(uDomains),
			DBCount:      len(uDBs),
			CanDelete:    canDelete,
			CanEditQuota: canEditQuota,
			ResellerName: resellerName,
		})
	}

	data := UsersPageData{
		BasePageData: NewBaseData(r, "Users & Quotas", "User, Reseller & RBAC Administration", "users", settings.Hostname, settings.Language),
		Users:        items,
		IsReseller:   sess.Role == model.RoleReseller,
		IsAdmin:      sess.Role == model.RoleAdmin,
	}

	_ = web.RenderTemplate(w, "users.html", data)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess.Role == model.RoleUser {
		http.Error(w, "Akses ditolak: End-User tidak memiliki izin untuk membuat akun", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/users?msg=Invalid+form+data&type=error", http.StatusSeeOther)
		return
	}

	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	email := strings.TrimSpace(r.FormValue("email"))
	password := strings.TrimSpace(r.FormValue("password"))
	role := model.Role(strings.TrimSpace(r.FormValue("role")))

	maxDiskMB, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("max_disk_mb")), 10, 64)
	maxRamMB, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("max_ram_mb")), 10, 64)
	maxDomains, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_domains")))
	maxDatabases, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_databases")))

	if maxDiskMB < 0 {
		maxDiskMB = 0
	}
	if maxRamMB < 0 {
		maxRamMB = 0
	}
	if maxDomains < 0 {
		maxDomains = 0
	}
	if maxDatabases < 0 {
		maxDatabases = 0
	}

	if username == "" || password == "" {
		http.Redirect(w, r, "/users?msg=Username+dan+password+wajib+diisi&type=error", http.StatusSeeOther)
		return
	}

	// Check if username already exists
	if existing, _ := h.repo.GetUserByUsername(username); existing != nil {
		http.Redirect(w, r, "/users?msg=Username+sudah+digunakan&type=error", http.StatusSeeOther)
		return
	}

	resellerID := ""
	if sess.Role == model.RoleReseller {
		// Reseller can ONLY create End-Users!
		role = model.RoleUser
		resellerID = sess.UserID

		// Reseller's End-Users have max 1 domain & max 1 database by requirement
		maxDomains = 1
		maxDatabases = 1

		// Ensure quota does not exceed reseller's own limits
		rq, _ := h.repo.GetQuota(sess.UserID)
		if rq != nil {
			if rq.MaxDiskMB > 0 && maxDiskMB > rq.MaxDiskMB {
				maxDiskMB = rq.MaxDiskMB
			}
			if rq.MaxRamMB > 0 && maxRamMB > rq.MaxRamMB {
				maxRamMB = rq.MaxRamMB
			}
		}
	} else if role == model.RoleUser {
		// End-Users created by admin also default to 1 domain and 1 database
		maxDomains = 1
		maxDatabases = 1
	}

	// Default fallback values if empty or zero
	if maxDiskMB <= 0 {
		if role == model.RoleReseller {
			maxDiskMB = 20480 // 20 GB default for reseller
		} else if role == model.RoleUser {
			maxDiskMB = 5120 // 5 GB default for end-user
		}
	}
	if maxRamMB <= 0 {
		if role == model.RoleReseller {
			maxRamMB = 4096 // 4 GB default for reseller
		} else if role == model.RoleUser {
			maxRamMB = 1024 // 1 GB default for end-user
		}
	}
	if role == model.RoleReseller && maxDomains <= 0 {
		maxDomains = 10
	}
	if role == model.RoleReseller && maxDatabases <= 0 {
		maxDatabases = 10
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Redirect(w, r, "/users?msg=Gagal+mengenkripsi+password&type=error", http.StatusSeeOther)
		return
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)

	homeDir := filepath.Join(h.cfg.WebRoot, username)

	// Create dedicated Linux system user (FastPanel style)
	_ = system.CreateSiteUser(r.Context(), username, username, homeDir)

	user := model.User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Email:        email,
		Role:         role,
		ResellerID:   resellerID,
		HomeDir:      homeDir,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.repo.UpsertUser(user); err != nil {
		http.Redirect(w, r, "/users?msg=Gagal+menyimpan+pengguna:+"+err.Error()+"&type=error", http.StatusSeeOther)
		return
	}

	// Save quotas
	quota := model.Quota{
		UserID:       id,
		MaxDiskMB:    maxDiskMB,
		MaxRamMB:     maxRamMB,
		MaxDomains:   maxDomains,
		MaxDatabases: maxDatabases,
	}
	_ = h.repo.SaveQuota(quota)

	http.Redirect(w, r, "/users?msg=Akun+"+username+"+berhasil+dibuat+dengan+kuota!&type=success", http.StatusSeeOther)
}

func (h *UserHandler) UpdateQuota(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess.Role == model.RoleUser {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/users?msg=Invalid+form+data&type=error", http.StatusSeeOther)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		http.Redirect(w, r, "/users?msg=User+ID+wajib+diisi&type=error", http.StatusSeeOther)
		return
	}

	targetUser, err := h.repo.GetUserByID(userID)
	if err != nil {
		http.Redirect(w, r, "/users?msg=User+tidak+ditemukan&type=error", http.StatusSeeOther)
		return
	}

	// Check permission: Admin can edit any user; Reseller can only edit their own end-users
	if sess.Role == model.RoleReseller && targetUser.ResellerID != sess.UserID && targetUser.ID != sess.UserID {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}

	maxDiskMB, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("max_disk_mb")), 10, 64)
	maxRamMB, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("max_ram_mb")), 10, 64)
	maxDomains, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_domains")))
	maxDatabases, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_databases")))

	if maxDiskMB < 0 {
		maxDiskMB = 0
	}
	if maxRamMB < 0 {
		maxRamMB = 0
	}
	if maxDomains < 0 {
		maxDomains = 0
	}
	if maxDatabases < 0 {
		maxDatabases = 0
	}

	// For End-Users, maxDomains and maxDatabases remain 1 by policy
	if targetUser.Role == model.RoleUser {
		maxDomains = 1
		maxDatabases = 1
	}

	q, _ := h.repo.GetQuota(targetUser.ID)
	if q == nil {
		q = &model.Quota{UserID: targetUser.ID}
	}
	q.MaxDiskMB = maxDiskMB
	q.MaxRamMB = maxRamMB
	q.MaxDomains = maxDomains
	q.MaxDatabases = maxDatabases

	_ = h.repo.SaveQuota(*q)

	// Optional password update
	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	passwordUpdated := false
	if newPassword != "" {
		hash, err := auth.HashPassword(newPassword)
		if err != nil {
			http.Redirect(w, r, "/users?msg=Gagal+mengenkripsi+password+baru&type=error", http.StatusSeeOther)
			return
		}
		targetUser.PasswordHash = hash
		if err := h.repo.UpsertUser(*targetUser); err != nil {
			http.Redirect(w, r, "/users?msg=Gagal+menyimpan+password+baru&type=error", http.StatusSeeOther)
			return
		}
		passwordUpdated = true
	}

	msg := "Pengaturan kuota untuk " + targetUser.Username + " berhasil diperbarui"
	if passwordUpdated {
		msg = "Pengaturan kuota dan password untuk " + targetUser.Username + " berhasil diperbarui"
	}

	http.Redirect(w, r, "/users?msg="+url.QueryEscape(msg)+"&type=success", http.StatusSeeOther)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	if sess.Role == model.RoleUser {
		http.Error(w, "Akses ditolak", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/users?msg=Invalid+request&type=error", http.StatusSeeOther)
		return
	}

	userID := strings.TrimSpace(r.FormValue("id"))
	if userID == "" || userID == sess.UserID {
		http.Redirect(w, r, "/users?msg=Tidak+dapat+menghapus+akun+sendiri&type=error", http.StatusSeeOther)
		return
	}

	targetUser, err := h.repo.GetUserByID(userID)
	if err != nil {
		http.Redirect(w, r, "/users?msg=User+tidak+ditemukan&type=error", http.StatusSeeOther)
		return
	}

	// Check permission
	if sess.Role == model.RoleReseller && targetUser.ResellerID != sess.UserID {
		http.Error(w, "Akses ditolak: Hanya dapat menghapus user milik Anda", http.StatusForbidden)
		return
	}

	_ = h.repo.DeleteUser(targetUser.ID)
	_ = system.DeleteSiteUser(r.Context(), targetUser.Username, targetUser.Username)

	http.Redirect(w, r, "/users?msg=Akun+"+targetUser.Username+"+berhasil+dihapus&type=success", http.StatusSeeOther)
}
