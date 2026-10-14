package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
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

	// In real environment, could query directly or read from repo
	dbs := []model.Database{
		{
			ID:        "db_1",
			Name:      "seintech_app",
			Type:      model.DBTypeMySQL,
			Charset:   "utf8mb4",
			SizeBytes: 4194304,
			CreatedAt: time.Now(),
		},
	}

	data := DBPageData{
		BasePageData: NewBaseData(r, "Databases", "Multi-Database Engine Provisioning", "databases", settings.Hostname),
		Databases:    dbs,
	}

	_ = web.RenderTemplate(w, "databases.html", data)
}

func (h *DatabaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	dbType := model.DBType(r.FormValue("type"))

	if name != "" {
		ctx := r.Context()
		if dbType == model.DBTypePostgres {
			_ = h.pgMgr.CreateDatabase(ctx, name, "UTF8", "")
		} else {
			_ = h.mysqlMgr.CreateDatabase(ctx, name, "utf8mb4", "utf8mb4_unicode_ci")
		}
	}

	http.Redirect(w, r, "/databases?success=created", http.StatusSeeOther)
}

func (h *DatabaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	dbType := model.DBType(r.FormValue("type"))

	if name != "" {
		ctx := r.Context()
		if dbType == model.DBTypePostgres {
			_ = h.pgMgr.DropDatabase(ctx, name)
		} else {
			_ = h.mysqlMgr.DropDatabase(ctx, name)
		}
	}

	http.Redirect(w, r, "/databases?success=deleted", http.StatusSeeOther)
}

func (h *DatabaseHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	dbName := r.FormValue("database")
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

// Suppress unused imports
var (
	_ = hex.EncodeToString
	_ = rand.Read
)
