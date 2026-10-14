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
