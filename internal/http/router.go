package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"

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

	// Reverse proxy for internal database web management tool (Adminer)
	dbToolsURL, _ := url.Parse("http://127.0.0.1:8089")
	dbProxy := httputil.NewSingleHostReverseProxy(dbToolsURL)
	origDirector := dbProxy.Director
	dbProxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = "127.0.0.1:8089"
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Real-IP", req.RemoteAddr)
	}
	dbProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Database Tool Unavailable</title><style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;text-align:center;padding:50px;color:#1e293b;background:#f8fafc;}h2{margin-bottom:8px;color:#0f172a;}p{color:#64748b;font-size:14px;}</style></head><body><h2>Database Web Tool Unavailable</h2><p>Layanan Nginx atau PHP-FPM belum aktif. Silakan jalankan layanan tersebut melalui menu Services &amp; Packages.</p></body></html>`))
	}

	r.Handle("/adminer", http.RedirectHandler("/adminer/", http.StatusMovedPermanently))
	r.Handle("/adminer/*", dbProxy)

	// Backwards compatibility: redirect legacy phpMyAdmin and phpPgAdmin paths to Adminer
	r.Handle("/phpmyadmin", http.RedirectHandler("/adminer/", http.StatusMovedPermanently))
	r.Handle("/phpmyadmin/*", http.RedirectHandler("/adminer/", http.StatusMovedPermanently))
	r.Handle("/phppgadmin", http.RedirectHandler("/adminer/", http.StatusMovedPermanently))
	r.Handle("/phppgadmin/*", http.RedirectHandler("/adminer/", http.StatusMovedPermanently))

	// Initialize Handlers
	authH := handler.NewAuthHandler(authMgr)
	dashH := handler.NewDashboardHandler(repo, sm)
	domainH := handler.NewDomainHandler(repo, registry, phpMgr, sslMgr, cfg, sm)
	fmH := handler.NewFileManagerHandler(repo, cfg)
	phpH := handler.NewPHPHandler(repo, phpMgr)
	dbH := handler.NewDatabaseHandler(repo, mysqlMgr, pgMgr, sm)
	sslH := handler.NewSSLHandler(repo, sslMgr)
	userH := handler.NewUserHandler(repo, cfg)
	settingsH := handler.NewSettingsHandler(repo, sm, sslMgr)
	servicesH := handler.NewServicesHandler(repo, sm, sslMgr)

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
		pr.Post("/domains/update", domainH.Update)
		pr.Post("/domains/update-engine", domainH.UpdateEngine)
		pr.Post("/domains/update-php", domainH.UpdatePHP)
		pr.Get("/domains/logs", domainH.GetLogs)
		pr.Post("/domains/clear-logs", domainH.ClearLogs)
		pr.Post("/domains/fix-permissions", domainH.FixPermissions)
		pr.Get("/domains/databases", domainH.GetDatabases)
		pr.Post("/domains/terminal", domainH.RunTerminal)
		pr.Get("/domains/vhost", domainH.GetVHost)
		pr.Post("/domains/vhost", domainH.SaveVHost)
		pr.Post("/domains/status", domainH.SetStatus)
		pr.Get("/domains/backup/download", domainH.DownloadBackup)
		pr.Post("/domains/backup/gdrive", domainH.SendGDriveBackup)
		pr.Post("/domains/delete", domainH.Delete)

		// File Manager
		pr.Get("/filemanager", fmH.List)
		pr.Get("/filemanager/read", fmH.Read)
		pr.Post("/filemanager/save", fmH.Save)
		pr.Get("/filemanager/download", fmH.Download)
		pr.Post("/filemanager/upload", fmH.Upload)
		pr.Post("/filemanager/delete", fmH.Delete)
		pr.Post("/filemanager/bulk-delete", fmH.BulkDelete)
		pr.Post("/filemanager/extract", fmH.Extract)
		pr.Post("/filemanager/compress", fmH.Compress)
		pr.Post("/filemanager/create-file", fmH.CreateFile)
		pr.Post("/filemanager/create-dir", fmH.CreateDir)
		pr.Post("/filemanager/rename", fmH.Rename)
		pr.Post("/filemanager/chmod", fmH.Chmod)
		pr.Post("/filemanager/fix-permissions", fmH.FixPermissions)

		// PHP Engines & Extensions & Configuration
		pr.Get("/php", phpH.List)
		pr.Get("/php/extensions", phpH.GetExtensions)
		pr.Post("/php/extension/install", phpH.InstallExtension)
		pr.Post("/php/extension/delete", phpH.DeleteExtension)
		pr.Get("/php/ini", phpH.GetIni)
		pr.Post("/php/ini/save", phpH.SaveIni)

		// Databases
		pr.Get("/databases", dbH.List)
		pr.Post("/databases/create", dbH.Create)
		pr.Post("/databases/setup", dbH.Setup)
		pr.Post("/databases/create-user", dbH.CreateUser)
		pr.Post("/databases/delete", dbH.Delete)
		pr.Get("/databases/export", dbH.Export)
		pr.Post("/databases/import", dbH.Import)
		pr.Get("/databases/sso", dbH.SSOLogin)
		pr.Post("/databases/set-password", dbH.SetPassword)
		pr.Post("/databases/grant-privileges", dbH.GrantPrivileges)
		pr.Post("/databases/fix-all-permissions", dbH.FixAllPermissions)

		// SSL
		pr.Get("/ssl", sslH.List)
		pr.Post("/ssl/issue", sslH.Issue)

		// Users & Resellers
		pr.Get("/users", userH.List)
		pr.Post("/users/create", userH.Create)
		pr.Post("/users/quota", userH.UpdateQuota)
		pr.Post("/users/delete", userH.Delete)

		// Admin-Only Routes
		pr.Group(func(admin chi.Router) {
			admin.Use(middleware.RequireRole(model.RoleAdmin))

			// Services & Package Installer
			admin.Get("/services", servicesH.List)
			admin.Post("/services/action", servicesH.ExecuteAction)
			admin.Post("/services/install", servicesH.StartInstallJob)
			admin.Post("/services/uninstall", servicesH.StartUninstallJob)
			admin.Get("/services/job/{jobID}", servicesH.GetJobStatus)

			admin.Get("/settings", settingsH.View)
			admin.Post("/settings/save", settingsH.Save)
			admin.Post("/settings/hosts/add", settingsH.AddHost)
			admin.Post("/settings/hosts/update", settingsH.UpdateHost)
			admin.Post("/settings/hosts/delete", settingsH.DeleteHost)
			admin.Post("/settings/hosts/ssl", settingsH.IssueSSL)
			admin.Post("/settings/proxy/sync", settingsH.SyncProxy)
		})
	})

	return r
}
