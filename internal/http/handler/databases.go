package handler

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/core/database"
	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type DatabaseHandler struct {
	repo     *storage.Repository
	mysqlMgr *database.MySQLManager
	pgMgr    *database.PostgresManager
	sm       *system.ServiceManager
}

func NewDatabaseHandler(repo *storage.Repository, mysqlMgr *database.MySQLManager, pgMgr *database.PostgresManager, sm *system.ServiceManager) *DatabaseHandler {
	return &DatabaseHandler{
		repo:     repo,
		mysqlMgr: mysqlMgr,
		pgMgr:    pgMgr,
		sm:       sm,
	}
}

type DBPageData struct {
	BasePageData
	Databases           []model.Database
	PhpMyAdminInstalled bool
	PhpPgAdminInstalled bool
	CanCreateDB         bool
}

func (h *DatabaseHandler) List(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	accessibleIDs := GetAccessibleUserIDs(sess, h.repo)

	settings, _ := h.repo.GetSettings()

	allDBs, err := h.repo.GetDatabases()
	if err != nil || len(allDBs) == 0 {
		allDBs = []model.Database{}
	}

	var dbs []model.Database
	for _, db := range allDBs {
		if accessibleIDs == nil || accessibleIDs[db.UserID] {
			dbs = append(dbs, db)
		}
	}

	ctx := r.Context()
	for i := range dbs {
		if dbs[i].Type == model.DBTypeMySQL {
			size, err := h.mysqlMgr.GetDatabaseSize(ctx, dbs[i].Name)
			if err == nil && size > 0 {
				dbs[i].SizeBytes = size
			}
		} else if dbs[i].Type == model.DBTypePostgres {
			size, err := h.pgMgr.GetDatabaseSize(ctx, dbs[i].Name)
			if err == nil && size > 0 {
				dbs[i].SizeBytes = size
			}
		}
	}

	// Automatically ensure permissions for accessible databases in background
	go func() {
		_ = h.pgMgr.EnsureAllPermissions(context.Background())
		for _, db := range dbs {
			if db.Type == model.DBTypePostgres && db.Username != "" && db.Name != "" {
				_ = h.pgMgr.GrantPrivileges(context.Background(), db.Username, db.Host, db.Name)
			}
		}
	}()

	canCreate := true
	if sess.Role == model.RoleUser && len(dbs) >= 1 {
		canCreate = false
	} else if sess.Role == model.RoleReseller {
		rq, _ := h.repo.GetQuota(sess.UserID)
		if rq != nil && rq.MaxDatabases > 0 && len(dbs) >= rq.MaxDatabases {
			canCreate = false
		}
	}

	data := DBPageData{
		BasePageData:        NewBaseData(r, "Databases", "Multi-Database Engine Provisioning", "databases", settings.Hostname, settings.Language),
		Databases:           dbs,
		PhpMyAdminInstalled: h.sm.IsInstalled("phpmyadmin"),
		PhpPgAdminInstalled: h.sm.IsInstalled("phppgadmin"),
		CanCreateDB:         canCreate,
	}

	_ = web.RenderTemplate(w, "databases.html", data)
}

func (h *DatabaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	sess := GetSession(r)

	// Check DB quota limits
	if sess.Role == model.RoleUser {
		allDBs, _ := h.repo.GetDatabases()
		count := 0
		for _, d := range allDBs {
			if d.UserID == sess.UserID {
				count++
			}
		}
		if count >= 1 {
			respondError(w, isAjax, "End-User hanya diperbolehkan memiliki 1 database.")
			return
		}
	} else if sess.Role == model.RoleReseller {
		rq, _ := h.repo.GetQuota(sess.UserID)
		if rq != nil && rq.MaxDatabases > 0 {
			accessibleIDs := GetAccessibleUserIDs(sess, h.repo)
			allDBs, _ := h.repo.GetDatabases()
			count := 0
			for _, d := range allDBs {
				if accessibleIDs[d.UserID] {
					count++
				}
			}
			if count >= rq.MaxDatabases {
				respondError(w, isAjax, fmt.Sprintf("Batas kuota database reseller (%d database) telah tercapai.", rq.MaxDatabases))
				return
			}
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	dbType := model.DBType(r.FormValue("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" {
		respondError(w, isAjax, "Nama database tidak boleh kosong")
		return
	}

	ctx := r.Context()
	var err error
	charset := "utf8mb4"
	if dbType == model.DBTypePostgres {
		charset = "UTF8"
		err = h.pgMgr.CreateDatabase(ctx, name, charset, "")
	} else {
		err = h.mysqlMgr.CreateDatabase(ctx, name, charset, "utf8mb4_unicode_ci")
	}

	if err != nil {
		respondError(w, isAjax, err.Error())
		return
	}

	_ = h.repo.UpsertDatabase(model.Database{
		ID:        "db_" + name,
		UserID:    sess.UserID,
		Name:      name,
		Type:      dbType,
		Charset:   charset,
		CreatedAt: time.Now(),
	})

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Database berhasil dibuat"})
		return
	}

	http.Redirect(w, r, "/databases?success=created", http.StatusSeeOther)
}

func (h *DatabaseHandler) Setup(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	sess := GetSession(r)

	// Check DB quota limits
	if sess.Role == model.RoleUser {
		allDBs, _ := h.repo.GetDatabases()
		count := 0
		for _, d := range allDBs {
			if d.UserID == sess.UserID {
				count++
			}
		}
		if count >= 1 {
			respondError(w, isAjax, "End-User hanya diperbolehkan memiliki 1 database.")
			return
		}
	} else if sess.Role == model.RoleReseller {
		rq, _ := h.repo.GetQuota(sess.UserID)
		if rq != nil && rq.MaxDatabases > 0 {
			accessibleIDs := GetAccessibleUserIDs(sess, h.repo)
			allDBs, _ := h.repo.GetDatabases()
			count := 0
			for _, d := range allDBs {
				if accessibleIDs[d.UserID] {
					count++
				}
			}
			if count >= rq.MaxDatabases {
				respondError(w, isAjax, fmt.Sprintf("Batas kuota database reseller (%d database) telah tercapai.", rq.MaxDatabases))
				return
			}
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	dbType := model.DBType(r.FormValue("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" || username == "" || password == "" {
		respondError(w, isAjax, "Database, User, dan Password wajib diisi")
		return
	}

	ctx := r.Context()
	charset := "utf8mb4"
	if dbType == model.DBTypePostgres {
		charset = "UTF8"
		if err := h.pgMgr.CreateDatabase(ctx, name, charset, ""); err != nil {
			respondError(w, isAjax, "Gagal membuat database: "+err.Error())
			return
		}
		if err := h.pgMgr.CreateUser(ctx, username, password, "localhost"); err != nil {
			respondError(w, isAjax, "Gagal membuat user DB: "+err.Error())
			return
		}
		_ = h.pgMgr.GrantPrivileges(ctx, username, "localhost", name)
	} else {
		if err := h.mysqlMgr.CreateDatabase(ctx, name, charset, "utf8mb4_unicode_ci"); err != nil {
			respondError(w, isAjax, "Gagal membuat database: "+err.Error())
			return
		}
		if err := h.mysqlMgr.CreateUser(ctx, username, password, "localhost"); err != nil {
			respondError(w, isAjax, "Gagal membuat user DB: "+err.Error())
			return
		}
		_ = h.mysqlMgr.GrantPrivileges(ctx, username, "localhost", name)
	}

	_ = h.repo.UpsertDatabase(model.Database{
		ID:        "db_" + name,
		UserID:    sess.UserID,
		Name:      name,
		Type:      dbType,
		Username:  username,
		Password:  password,
		Host:      "localhost",
		Charset:   charset,
		CreatedAt: time.Now(),
	})

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"message":  fmt.Sprintf("Database %s dan user %s berhasil disetup!", name, username),
			"database": name,
			"username": username,
		})
		return
	}

	http.Redirect(w, r, "/databases?success=setup_complete", http.StatusSeeOther)
}

func (h *DatabaseHandler) isDBAccessible(sess *auth.Session, dbName string) bool {
	if sess == nil || sess.Role == model.RoleAdmin {
		return true
	}
	dbs, _ := h.repo.GetDatabases()
	for _, d := range dbs {
		if strings.EqualFold(d.Name, dbName) {
			return IsUserAccessible(sess, h.repo, d.UserID)
		}
	}
	return false
}

func (h *DatabaseHandler) Export(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	dbType := model.DBType(r.URL.Query().Get("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" {
		http.Error(w, "Nama database tidak valid", http.StatusBadRequest)
		return
	}

	if !h.isDBAccessible(sess, name) {
		http.Error(w, "Akses ditolak: Anda tidak memiliki akses ke database ini", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/sql")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_%s.sql\"", name, time.Now().Format("20060102_150405")))

	ctx := r.Context()
	var cmd *exec.Cmd
	if dbType == model.DBTypePostgres {
		cmd = exec.CommandContext(ctx, "su", "-", "postgres", "-c", fmt.Sprintf("pg_dump %s", name))
	} else {
		cmd = exec.CommandContext(ctx, "mysqldump", "-u", "root", name)
	}

	cmd.Stdout = w
	_ = cmd.Run()
}

func (h *DatabaseHandler) Import(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	sess := GetSession(r)
	name := strings.TrimSpace(r.FormValue("name"))
	dbType := model.DBType(r.FormValue("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" {
		respondError(w, isAjax, "Nama database tidak ditentukan")
		return
	}

	if !h.isDBAccessible(sess, name) {
		respondError(w, isAjax, "Akses ditolak: Anda tidak memiliki izin untuk mengimpor ke database ini")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, isAjax, "Gagal membaca file SQL: "+err.Error())
		return
	}
	defer file.Close()

	var reader io.Reader = file
	if strings.HasSuffix(strings.ToLower(header.Filename), ".gz") {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			respondError(w, isAjax, "Gagal membaca file gz: "+err.Error())
			return
		}
		defer gzr.Close()
		reader = gzr
	}

	ctx := r.Context()
	var cmd *exec.Cmd
	if dbType == model.DBTypePostgres {
		cmd = exec.CommandContext(ctx, "su", "-", "postgres", "-c", fmt.Sprintf("psql -d %s", name))
	} else {
		cmd = exec.CommandContext(ctx, "mysql", "-u", "root", name)
	}

	cmd.Stdin = reader
	out, err := cmd.CombinedOutput()
	if err != nil {
		respondError(w, isAjax, fmt.Sprintf("Import error: %v (%s)", err, strings.TrimSpace(string(out))))
		return
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Database berhasil diimpor!"})
		return
	}

	http.Redirect(w, r, "/databases?success=imported", http.StatusSeeOther)
}

func (h *DatabaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	sess := GetSession(r)
	name := strings.TrimSpace(r.FormValue("name"))
	dbType := model.DBType(r.FormValue("type"))

	if name != "" {
		if !h.isDBAccessible(sess, name) {
			respondError(w, isAjax, "Akses ditolak: Anda tidak memiliki izin untuk menghapus database ini")
			return
		}

		ctx := r.Context()
		if dbType == model.DBTypePostgres {
			_ = h.pgMgr.DropDatabase(ctx, name)
		} else {
			_ = h.mysqlMgr.DropDatabase(ctx, name)
		}
		_ = h.repo.DeleteDatabase(name)
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Database berhasil dihapus"})
		return
	}

	http.Redirect(w, r, "/databases?success=deleted", http.StatusSeeOther)
}

func (h *DatabaseHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	dbName := strings.TrimSpace(r.FormValue("database"))
	dbType := model.DBType(r.FormValue("type"))

	if sess.Role != model.RoleAdmin && dbName == "" {
		http.Error(w, "Akses ditolak: Database harus ditentukan", http.StatusBadRequest)
		return
	}

	if dbName != "" && !h.isDBAccessible(sess, dbName) {
		http.Error(w, "Akses ditolak: Anda tidak memiliki akses ke database ini", http.StatusForbidden)
		return
	}

	if username != "" && password != "" {
		ctx := r.Context()
		if dbType == model.DBTypePostgres {
			_ = h.pgMgr.CreateUser(ctx, username, password, "localhost")
			if dbName != "" {
				_ = h.pgMgr.GrantPrivileges(ctx, username, "localhost", dbName)
			}
		} else {
			_ = h.mysqlMgr.CreateUser(ctx, username, password, "localhost")
			if dbName != "" {
				_ = h.mysqlMgr.GrantPrivileges(ctx, username, "localhost", dbName)
			}
		}

		if dbName != "" {
			dbs, _ := h.repo.GetDatabases()
			for _, d := range dbs {
				if strings.EqualFold(d.Name, dbName) {
					d.Username = username
					d.Password = password
					_ = h.repo.UpsertDatabase(d)
					break
				}
			}
		}
	}

	http.Redirect(w, r, "/databases?success=user_created", http.StatusSeeOther)
}

// SSOLogin generates a secure one-time token and redirects the browser directly into phpMyAdmin or phpPgAdmin.
func (h *DatabaseHandler) SSOLogin(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	name := strings.TrimSpace(r.URL.Query().Get("name"))

	if !h.isDBAccessible(sess, name) {
		http.Redirect(w, r, "/databases?error=access_denied", http.StatusSeeOther)
		return
	}

	dbs, _ := h.repo.GetDatabases()
	var targetDB *model.Database
	for i := range dbs {
		if strings.EqualFold(dbs[i].Name, name) {
			targetDB = &dbs[i]
			break
		}
	}

	if targetDB == nil {
		http.Redirect(w, r, "/databases?error=db_not_found", http.StatusSeeOther)
		return
	}

	// If database record does not have a saved password yet, redirect with modal trigger
	if targetDB.Password == "" {
		http.Redirect(w, r, fmt.Sprintf("/databases?prompt_pwd=1&db=%s&user=%s&type=%s", url.QueryEscape(targetDB.Name), url.QueryEscape(targetDB.Username), url.QueryEscape(string(targetDB.Type))), http.StatusSeeOther)
		return
	}

	// Generate secure one-time SSO token
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	tokenData := map[string]interface{}{
		"user":    targetDB.Username,
		"pass":    targetDB.Password,
		"db":      targetDB.Name,
		"type":    string(targetDB.Type),
		"expires": time.Now().Add(180 * time.Second).Unix(),
	}

	tokenDir := "/var/lib/seintech/pma_tokens"
	_ = os.MkdirAll(tokenDir, 0777)
	_ = os.Chmod(tokenDir, 0777)

	tokenBytesData, _ := json.Marshal(tokenData)
	tokenFile := filepath.Join(tokenDir, token+".json")
	_ = os.WriteFile(tokenFile, tokenBytesData, 0666)
	_ = os.Chmod(tokenFile, 0666)

	if targetDB.Type == model.DBTypePostgres {
		http.Redirect(w, r, fmt.Sprintf("/phppgadmin/sso.php?token=%s", token), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/phpmyadmin/sso.php?token=%s", token), http.StatusSeeOther)
	}
}

// SetPassword updates database password in database system and repository for seamless auto-login.
func (h *DatabaseHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	name := strings.TrimSpace(r.FormValue("name"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	syncServer := r.FormValue("sync") == "1" || r.FormValue("sync") == "true"

	if name == "" || password == "" {
		http.Error(w, "Nama database dan password wajib diisi", http.StatusBadRequest)
		return
	}

	if !h.isDBAccessible(sess, name) {
		http.Error(w, "Akses ditolak: Anda tidak memiliki akses ke database ini", http.StatusForbidden)
		return
	}

	dbs, _ := h.repo.GetDatabases()
	var targetDB *model.Database
	for i := range dbs {
		if strings.EqualFold(dbs[i].Name, name) {
			targetDB = &dbs[i]
			break
		}
	}

	if targetDB == nil {
		http.Error(w, "Database tidak ditemukan", http.StatusNotFound)
		return
	}

	if username != "" {
		targetDB.Username = username
	} else if targetDB.Username == "" {
		targetDB.Username = targetDB.Name
	}
	targetDB.Password = password

	ctx := r.Context()
	// Sync user password directly on database engine (MariaDB or PostgreSQL)
	if syncServer && targetDB.Username != "" {
		if targetDB.Type == model.DBTypePostgres {
			_ = h.pgMgr.ChangePassword(ctx, targetDB.Username, "localhost", password)
		} else {
			_ = h.mysqlMgr.ChangePassword(ctx, targetDB.Username, "localhost", password)
		}
	}

	_ = h.repo.UpsertDatabase(*targetDB)

	// Return direct SSO URL for instantaneous browser redirect
	ssoURL := fmt.Sprintf("/databases/sso?name=%s&type=%s", url.QueryEscape(targetDB.Name), url.QueryEscape(string(targetDB.Type)))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"sso_url": ssoURL,
	})
}

func (h *DatabaseHandler) GrantPrivileges(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	name := strings.TrimSpace(r.FormValue("name"))
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")

	if !h.isDBAccessible(sess, name) {
		respondError(w, isAjax, "Akses ditolak: Anda tidak memiliki akses ke database ini")
		return
	}

	dbs, err := h.repo.GetDatabases()
	if err != nil {
		respondError(w, isAjax, "Gagal mengambil daftar database: "+err.Error())
		return
	}

	var targetDB *model.Database
	for _, d := range dbs {
		if strings.EqualFold(d.Name, name) {
			targetDB = &d
			break
		}
	}

	if targetDB == nil {
		respondError(w, isAjax, "Database tidak ditemukan")
		return
	}

	ctx := r.Context()
	user := targetDB.Username
	if user == "" {
		user = targetDB.Name
	}

	if targetDB.Type == model.DBTypePostgres {
		if err := h.pgMgr.GrantPrivileges(ctx, user, targetDB.Host, targetDB.Name); err != nil {
			respondError(w, isAjax, "Gagal memperbarui hak akses PostgreSQL: "+err.Error())
			return
		}
	} else {
		if err := h.mysqlMgr.GrantPrivileges(ctx, user, targetDB.Host, targetDB.Name); err != nil {
			respondError(w, isAjax, "Gagal memperbarui hak akses MySQL: "+err.Error())
			return
		}
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Hak akses user '%s' pada database '%s' (schema & tables) berhasil diperbarui!", user, targetDB.Name),
		})
		return
	}

	http.Redirect(w, r, "/databases?success=permissions_granted", http.StatusSeeOther)
}

func (h *DatabaseHandler) FixAllPermissions(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")

	if sess.Role != model.RoleAdmin {
		respondError(w, isAjax, "Akses ditolak: Hanya Administrator yang dapat menjalankan Fix All Permissions")
		return
	}

	ctx := r.Context()

	// Fix all PostgreSQL databases in engine
	_ = h.pgMgr.EnsureAllPermissions(ctx)

	// Fix registered databases in repo (both MySQL and PostgreSQL)
	dbs, _ := h.repo.GetDatabases()
	for _, db := range dbs {
		u := db.Username
		if u == "" {
			u = db.Name
		}
		if db.Type == model.DBTypePostgres {
			_ = h.pgMgr.GrantPrivileges(ctx, u, db.Host, db.Name)
		} else if db.Type == model.DBTypeMySQL {
			_ = h.mysqlMgr.GrantPrivileges(ctx, u, db.Host, db.Name)
		}
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Semua izin database (schema public, ownership, dan privileges) berhasil diperbarui secara otomatis!",
		})
		return
	}

	http.Redirect(w, r, "/databases?success=all_permissions_fixed", http.StatusSeeOther)
}

func respondError(w http.ResponseWriter, isAjax bool, msg string) {
	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": msg})
		return
	}
	http.Error(w, msg, http.StatusInternalServerError)
}
