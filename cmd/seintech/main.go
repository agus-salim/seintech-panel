package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/auth"
	"github.com/seintech/panel/internal/core/database"
	"github.com/seintech/panel/internal/core/php"
	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/core/webserver"
	phttp "github.com/seintech/panel/internal/http"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
)

var (
	Version   = "1.0.0"
	BuildTime = "2026-10-14"
)

func main() {
	var (
		portFlag    = flag.Int("port", 8443, "Panel HTTPS listening port")
		addrFlag    = flag.String("addr", "0.0.0.0", "Panel listening address")
		devFlag     = flag.Bool("dev", false, "Run in development mode")
		versionFlag = flag.Bool("version", false, "Print version information")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Seintech Hosting Panel v%s (%s)\n", Version, BuildTime)
		os.Exit(0)
	}

	if *devFlag {
		os.Setenv("SEINTECH_DEV", "1")
	}

	cfg := config.LoadConfig()
	if *portFlag != 8443 {
		cfg.Port = *portFlag
	}
	if *addrFlag != "0.0.0.0" {
		cfg.ListenAddr = *addrFlag
	}

	log.Printf("[Seintech] Initializing Seintech Hosting Panel v%s...", Version)
	log.Printf("[Seintech] Data directory: %s", cfg.DataDir)
	log.Printf("[Seintech] Web root directory: %s", cfg.WebRoot)

	// Self-heal any nested /var/www/var/www artifacts from previous path resolution
	healNestedWebRoot(cfg.WebRoot)

	// Sync bundled ionCube loaders to /usr/local/ioncube
	syncIonCubeLoaders()

	// Initialize Storage Repository
	repo, err := storage.NewRepository(cfg.DataDir)
	if err != nil {
		log.Fatalf("[Seintech] Failed to initialize storage repository: %v", err)
	}

	// Bootstrap Admin Account if no users exist
	bootstrapAdmin(repo, cfg)

	// Initialize Subsystems
	sm := system.NewServiceManager()
	wsRegistry := webserver.NewRegistry(sm)
	phpMgr := php.NewPHPManager(sm)
	sslMgr := ssl.NewSSLManager(repo, wsRegistry, cfg.SSLBase, cfg.ACMERoot)
	mysqlMgr := database.NewMySQLManager(sm)
	pgMgr := database.NewPostgresManager(sm)
	authMgr := auth.NewAuthManager(repo)

	// Ensure system APT repositories, Apache backend config, and cleanup deprecated repos on startup
	go func() {
		repoCtx, repoCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer repoCancel()
		sm.EnsureSystemRepositories(repoCtx)
		sm.EnsureApacheConfiguration(repoCtx)
		sm.CleanupDeprecatedRepositories(repoCtx)

		// Ensure .htaccess exists for all active domains
		if domains, err := repo.GetDomains(); err == nil {
			for _, d := range domains {
				if d.DocumentRoot != "" {
					system.EnsureDomainHtaccess(d.DocumentRoot, d.SiteUser, d.SiteGroup)
				}
			}
		}
	}()

	// Context for graceful shutdown & background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Background SSL Renewal Worker
	sslMgr.StartAutoRenewalWorker(ctx)

	// Setup Router
	router := phttp.NewRouter(
		cfg,
		repo,
		authMgr,
		sm,
		wsRegistry,
		phpMgr,
		sslMgr,
		mysqlMgr,
		pgMgr,
	)

	// Ensure Panel Self-Signed SSL Certificate exists
	panelCert := filepath.Join(cfg.SSLBase, "panel", "cert.pem")
	panelKey := filepath.Join(cfg.SSLBase, "panel", "key.pem")
	if _, err := os.Stat(panelCert); os.IsNotExist(err) {
		log.Println("[Seintech] Generating self-signed TLS certificate for panel access...")
		_ = sslMgr.GenerateSelfSigned("localhost", panelCert, panelKey)
	}

	// Ensure Nginx reverse proxy is configured for panel domains (port 443 / 80)
	if err := sslMgr.SyncPanelNginxReverseProxy(ctx); err != nil {
		log.Printf("[Seintech] Notice: Nginx reverse proxy sync: %v", err)
	}

	listenTarget := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.Port)

	// Prepare TLS Server
	server := &http.Server{
		Addr:         listenTarget,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				if hello != nil && hello.ServerName != "" {
					cleanName := strings.ToLower(strings.TrimSpace(hello.ServerName))
					// Check panel-specific host certificate
					hCert := filepath.Join(cfg.SSLBase, "panel", cleanName, "fullchain.pem")
					hKey := filepath.Join(cfg.SSLBase, "panel", cleanName, "privkey.pem")
					if _, err := os.Stat(hCert); err == nil {
						if kp, err := tls.LoadX509KeyPair(hCert, hKey); err == nil {
							return &kp, nil
						}
					}
					// Check domain certificate
					gCert := filepath.Join(cfg.SSLBase, cleanName, "fullchain.pem")
					gKey := filepath.Join(cfg.SSLBase, cleanName, "privkey.pem")
					if _, err := os.Stat(gCert); err == nil {
						if kp, err := tls.LoadX509KeyPair(gCert, gKey); err == nil {
							return &kp, nil
						}
					}
				}
				// Default fallback to panel certificate
				kp, err := tls.LoadX509KeyPair(panelCert, panelKey)
				if err != nil {
					return nil, err
				}
				return &kp, nil
			},
		},
	}

	// Channel to capture OS interrupt signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Seintech] Control Panel running securely at https://%s:%d", cfg.ListenAddr, cfg.Port)
		log.Printf("[Seintech] Web interface ready. Press Ctrl+C to terminate.")

		if err := server.ListenAndServeTLS(panelCert, panelKey); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Seintech] Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal
	<-stopChan
	log.Println("[Seintech] Shutdown signal received. Performing graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Seintech] Graceful shutdown error: %v", err)
	}

	log.Println("[Seintech] Panel service terminated cleanly.")
}

func bootstrapAdmin(repo *storage.Repository, cfg *config.Config) {
	users, err := repo.GetUsers()
	if err == nil && len(users) > 0 {
		return
	}

	log.Println("[Seintech] No users detected. Bootstrapping default administrator...")
	hash, err := auth.HashPassword("admin123")
	if err != nil {
		log.Fatalf("Failed to hash default password: %v", err)
	}

	admin := model.User{
		ID:           "usr_root_admin",
		Username:     "admin",
		PasswordHash: hash,
		Email:        "admin@seintech.local",
		Role:         model.RoleAdmin,
		HomeDir:      filepath.Join(cfg.WebRoot, "admin"),
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := repo.UpsertUser(admin); err != nil {
		log.Fatalf("Failed to save default admin user: %v", err)
	}

	log.Println("=================================================================")
	log.Println(" [SEINTECH] DEFAULT CREDENTIALS CREATED:")
	log.Println("  Username: admin")
	log.Println("  Password: admin123")
	log.Println("  Please change this password immediately after first login!")
	log.Println("=================================================================")
}

func healNestedWebRoot(webRoot string) {
	nested := filepath.Join(webRoot, "var", "www")
	if fi, err := os.Stat(nested); err == nil && fi.IsDir() {
		log.Printf("[Seintech] Found nested web root at %s, migrating files to %s...", nested, webRoot)
		_ = filepath.Walk(nested, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == nested {
				return nil
			}
			rel, err := filepath.Rel(nested, path)
			if err != nil {
				return nil
			}
			dest := filepath.Join(webRoot, rel)
			if info.IsDir() {
				_ = os.MkdirAll(dest, 0755)
			} else {
				_ = os.MkdirAll(filepath.Dir(dest), 0755)
				data, err := os.ReadFile(path)
				if err == nil {
					_ = os.WriteFile(dest, data, info.Mode())
				}
			}
			return nil
		})
		_ = os.RemoveAll(filepath.Join(webRoot, "var"))
		log.Printf("[Seintech] Migration of nested web root complete.")
	}
}

func syncIonCubeLoaders() {
	destDir := "/usr/local/ioncube"
	candidates := []string{
		"/root/seintech-panel/ioncube",
		"./ioncube",
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "ioncube"),
			filepath.Join(filepath.Dir(exe), "..", "ioncube"),
		)
	}

	var srcDir string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			srcDir = c
			break
		}
	}

	if srcDir == "" {
		return
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return
	}

	_ = os.MkdirAll(destDir, 0755)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "ioncube_loader_lin_") && strings.HasSuffix(entry.Name(), ".so") {
			srcFile := filepath.Join(srcDir, entry.Name())
			dstFile := filepath.Join(destDir, entry.Name())
			if _, err := os.Stat(dstFile); os.IsNotExist(err) {
				if content, err := os.ReadFile(srcFile); err == nil {
					_ = os.WriteFile(dstFile, content, 0755)
					log.Printf("[Seintech] Installed ionCube Loader: %s -> %s", entry.Name(), dstFile)
				}
			}
		}
	}
}

