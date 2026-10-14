package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/core/database"
	"github.com/seintech/panel/internal/core/php"
	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/http/handler"
	"github.com/seintech/panel/internal/http/middleware"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

func NewRouter(
	cfg *config.Config,
	repo *storage.Repository,
	authMgr *auth.AuthManager,
	sm *system.ServiceManager,
	registry *webserver.Registry,
	phpMgr *php.PHPManager,
	sslMgr *ssl.SSLManager,
	mysqlMgr *database.MySQLManager,
	pgMgr *database.PostgresManager,
) http.Handler {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)

	// Static Files from embedded FS
	staticFS, err := web.GetStaticFS()
	if err == nil {
		r.Handle("/static/*", http.StripPrefix("/static/", staticFS))
	}

	// ACME Challenge Server for Let's Encrypt HTTP-01
	r.Handle("/.well-known/acme-challenge/*", http.StripPrefix("/.well-known/acme-challenge/", http.FileServer(http.Dir(cfg.ACMERoot))))

	// Initialize Handlers
	authH := handler.NewAuthHandler(authMgr)
	dashH := handler.NewDashboardHandler(repo, sm)
	domainH := handler.NewDomainHandler(repo, registry, phpMgr, sslMgr, cfg)
	fmH := handler.NewFileManagerHandler(repo, cfg)
	phpH := handler.NewPHPHandler(repo, phpMgr)
	dbH := handler.NewDatabaseHandler(repo, mysqlMgr, pgMgr)
	sslH := handler.NewSSLHandler(repo, sslMgr)
	userH := handler.NewUserHandler(repo, cfg)
	settingsH := handler.NewSettingsHandler(repo, sm)
	servicesH := handler.NewServicesHandler(repo, sm)

	// Public Routes
	r.Get("/login", authH.ShowLogin)
	r.Post("/login", authH.Login)
	r.Post("/logout", authH.Logout)

	// Authenticated Routes
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.RequireAuth(authMgr))

		pr.Get("/", dashH.ServeHTTP)

		// Domains & VHosts
		pr.Get("/domains", domainH.List)
		pr.Post("/domains/create", domainH.Create)
		pr.Post("/domains/delete", domainH.Delete)

		// File Manager
		pr.Get("/filemanager", fmH.List)
		pr.Get("/filemanager/read", fmH.Read)
		pr.Post("/filemanager/save", fmH.Save)
		pr.Get("/filemanager/download", fmH.Download)
		pr.Post("/filemanager/upload", fmH.Upload)
		pr.Post("/filemanager/delete", fmH.Delete)
		pr.Post("/filemanager/create-file", fmH.CreateFile)
		pr.Post("/filemanager/create-dir", fmH.CreateDir)
		pr.Post("/filemanager/rename", fmH.Rename)
		pr.Post("/filemanager/chmod", fmH.Chmod)

		// Services & Package Installer
		pr.Get("/services", servicesH.List)
		pr.Post("/services/action", servicesH.ExecuteAction)
		pr.Post("/services/install", servicesH.StartInstallJob)
		pr.Post("/services/uninstall", servicesH.StartUninstallJob)
		pr.Get("/services/job/{jobID}", servicesH.GetJobStatus)

		// PHP Engines
		pr.Get("/php", phpH.List)

		// Databases
		pr.Get("/databases", dbH.List)
		pr.Post("/databases/create", dbH.Create)
		pr.Post("/databases/create-user", dbH.CreateUser)
		pr.Post("/databases/delete", dbH.Delete)

		// SSL
		pr.Get("/ssl", sslH.List)
		pr.Post("/ssl/issue", sslH.Issue)

		// Users & Resellers
		pr.Get("/users", userH.List)
		pr.Post("/users/create", userH.Create)

		// Admin-Only Routes
		pr.Group(func(admin chi.Router) {
			admin.Use(middleware.RequireRole(model.RoleAdmin))

			admin.Get("/settings", settingsH.View)
			admin.Post("/settings/save", settingsH.Save)
		})
	})

	return r
}
