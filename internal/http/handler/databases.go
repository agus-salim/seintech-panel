package handler

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/seintech/panel/internal/core/database"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type DatabaseHandler struct {
	repo     *storage.Repository
	mysqlMgr *database.MySQLManager
	pgMgr    *database.PostgresManager
}

func NewDatabaseHandler(repo *storage.Repository, mysqlMgr *database.MySQLManager, pgMgr *database.PostgresManager) *DatabaseHandler {
	return &DatabaseHandler{
		repo:     repo,
		mysqlMgr: mysqlMgr,
		pgMgr:    pgMgr,
	}
}

type DBPageData struct {
	BasePageData
	Databases []model.Database
}

func (h *DatabaseHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()

	dbs, err := h.repo.GetDatabases()
	if err != nil || len(dbs) == 0 {
		dbs = []model.Database{}
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

	data := DBPageData{
		BasePageData: NewBaseData(r, "Databases", "Multi-Database Engine Provisioning", "databases", settings.Hostname),
		Databases:    dbs,
	}

	_ = web.RenderTemplate(w, "databases.html", data)
}

func (h *DatabaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	name := strings.TrimSpace(r.FormValue("name"))
	dbType := model.DBType(r.FormValue("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Nama database tidak boleh kosong"})
			return
		}
		http.Redirect(w, r, "/databases?error=empty_name", http.StatusSeeOther)
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
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Redirect(w, r, "/databases?error="+err.Error(), http.StatusSeeOther)
		return
	}

	_ = h.repo.UpsertDatabase(model.Database{
		ID:        "db_" + name,
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
	name := strings.TrimSpace(r.FormValue("name"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	dbType := model.DBType(r.FormValue("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" || username == "" || password == "" {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Database, User, dan Password wajib diisi"})
			return
		}
		http.Redirect(w, r, "/domains?error=missing_fields", http.StatusSeeOther)
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
		Name:      name,
		Type:      dbType,
		Username:  username,
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

func (h *DatabaseHandler) Export(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	dbType := model.DBType(r.URL.Query().Get("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" {
		http.Error(w, "Nama database tidak valid", http.StatusBadRequest)
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
	name := strings.TrimSpace(r.FormValue("name"))
	dbType := model.DBType(r.FormValue("type"))
	if dbType == "" {
		dbType = model.DBTypeMySQL
	}

	if name == "" {
		respondError(w, isAjax, "Nama database tidak ditentukan")
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
	name := strings.TrimSpace(r.FormValue("name"))
	dbType := model.DBType(r.FormValue("type"))

	if name != "" {
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
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	dbName := strings.TrimSpace(r.FormValue("database"))
	dbType := model.DBType(r.FormValue("type"))

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
	}

	http.Redirect(w, r, "/databases?success=user_created", http.StatusSeeOther)
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
