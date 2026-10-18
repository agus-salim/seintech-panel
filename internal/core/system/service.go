package system

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/seintech/panel/web"
)

var (
	ErrDisallowedService = errors.New("service name is not in the allowed whitelist")
	ErrDisallowedAction  = errors.New("service action is not in the allowed whitelist")
)

// AllowedServices whitelist to prevent arbitrary command execution via systemctl
var AllowedServices = map[string]bool{
	"nginx":       true,
	"apache2":     true,
	"mariadb":     true,
	"mysql":       true,
	"postgresql":  true,
	"php7.4-fpm":  true,
	"php8.0-fpm":  true,
	"php8.1-fpm":  true,
	"php8.2-fpm":  true,
	"php8.3-fpm":  true,
	"php8.4-fpm":  true,
	"php8.5-fpm":  true,
	"adminer":     true,
	"seintech":    true,
}

var AllowedActions = map[string]bool{
	"start":     true,
	"stop":      true,
	"restart":   true,
	"reload":    true,
	"status":    true,
	"is-active": true,
	"enable":    true,
	"disable":   true,
}

type ServiceState struct {
	Name    string `json:"name"`
	Active  bool   `json:"active"`
	Status  string `json:"status"` // running, stopped, failed, unknown
	Uptime  string `json:"uptime,omitempty"`
}

type ServiceDetail struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"` // webserver, database, php
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	Active      bool   `json:"active"`
	Status      string `json:"status"` // running, stopped, not_installed
	Packages    string `json:"packages"`
	Ports       string `json:"ports,omitempty"`
}

type ServiceDefinition struct {
	Name        string
	DisplayName string
	Category    string
	Description string
	CheckPaths  []string
	AptPackages []string
	Ports       string
}

var Catalog = []ServiceDefinition{
	// Web Servers
	{
		Name:        "nginx",
		DisplayName: "Nginx Web Server",
		Category:    "webserver",
		Description: "High-performance HTTP server, reverse proxy, and IMAP/POP3 proxy server.",
		CheckPaths:  []string{"/usr/sbin/nginx"},
		AptPackages: []string{"nginx"},
		Ports:       "HTTP :80 • HTTPS :443 (Edge Gateway)",
	},
	{
		Name:        "apache2",
		DisplayName: "Apache 2.4 Server",
		Category:    "webserver",
		Description: "Reliable, open-source HTTP server with modular proxy and rewrite capabilities.",
		CheckPaths:  []string{"/usr/sbin/apache2"},
		AptPackages: []string{"apache2", "apache2-utils"},
		Ports:       "Port :8080",
	},

	// Databases
	{
		Name:        "mariadb",
		DisplayName: "MariaDB / MySQL Server",
		Category:    "database",
		Description: "Fast, robust, and scalable relational database system compatible with MySQL.",
		CheckPaths:  []string{"/usr/sbin/mariadbd", "/usr/sbin/mysqld", "/usr/bin/mariadb"},
		AptPackages: []string{"mariadb-server", "mariadb-client"},
	},
	{
		Name:        "postgresql",
		DisplayName: "PostgreSQL Server",
		Category:    "database",
		Description: "Powerful, enterprise-class open source object-relational database system.",
		CheckPaths:  []string{"/usr/lib/postgresql", "/usr/bin/psql"},
		AptPackages: []string{"postgresql", "postgresql-contrib"},
	},

	// PHP Engines
	{
		Name:        "php7.4-fpm",
		DisplayName: "PHP 7.4 FPM",
		Category:    "php",
		Description: "Legacy PHP FastCGI Process Manager for older web applications.",
		CheckPaths:  []string{"/usr/sbin/php-fpm7.4"},
		AptPackages: []string{"php7.4-fpm", "php7.4-cli", "php7.4-mysql", "php7.4-pgsql", "php7.4-curl", "php7.4-gd", "php7.4-mbstring", "php7.4-xml", "php7.4-zip"},
	},
	{
		Name:        "php8.0-fpm",
		DisplayName: "PHP 8.0 FPM",
		Category:    "php",
		Description: "PHP 8.0 FastCGI Process Manager with JIT compiler support.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.0"},
		AptPackages: []string{"php8.0-fpm", "php8.0-cli", "php8.0-mysql", "php8.0-pgsql", "php8.0-curl", "php8.0-gd", "php8.0-mbstring", "php8.0-xml", "php8.0-zip"},
	},
	{
		Name:        "php8.1-fpm",
		DisplayName: "PHP 8.1 FPM",
		Category:    "php",
		Description: "PHP 8.1 with enums, readonly properties, and first-class callables.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.1"},
		AptPackages: []string{"php8.1-fpm", "php8.1-cli", "php8.1-mysql", "php8.1-pgsql", "php8.1-curl", "php8.1-gd", "php8.1-mbstring", "php8.1-xml", "php8.1-zip"},
	},
	{
		Name:        "php8.2-fpm",
		DisplayName: "PHP 8.2 FPM",
		Category:    "php",
		Description: "PHP 8.2 stable release with readonly classes and performance improvements.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.2"},
		AptPackages: []string{"php8.2-fpm", "php8.2-cli", "php8.2-mysql", "php8.2-pgsql", "php8.2-curl", "php8.2-gd", "php8.2-mbstring", "php8.2-xml", "php8.2-zip"},
	},
	{
		Name:        "php8.3-fpm",
		DisplayName: "PHP 8.3 FPM",
		Category:    "php",
		Description: "PHP 8.3 current standard with typed class constants and json_validate.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.3"},
		AptPackages: []string{"php8.3-fpm", "php8.3-cli", "php8.3-mysql", "php8.3-pgsql", "php8.3-curl", "php8.3-gd", "php8.3-mbstring", "php8.3-xml", "php8.3-zip"},
	},
	{
		Name:        "php8.4-fpm",
		DisplayName: "PHP 8.4 FPM",
		Category:    "php",
		Description: "PHP 8.4 latest modern runtime with property hooks and asymmetric visibility.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.4"},
		AptPackages: []string{"php8.4-fpm", "php8.4-cli", "php8.4-mysql", "php8.4-pgsql", "php8.4-curl", "php8.4-gd", "php8.4-mbstring", "php8.4-xml", "php8.4-zip"},
	},
	{
		Name:        "php8.5-fpm",
		DisplayName: "PHP 8.5 FPM",
		Category:    "php",
		Description: "PHP 8.5 developer preview runtime for cutting edge evaluation.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.5"},
		AptPackages: []string{"php8.5-fpm", "php8.5-cli", "php8.5-mysql", "php8.5-pgsql", "php8.5-curl", "php8.5-gd", "php8.5-mbstring", "php8.5-xml", "php8.5-zip"},
	},

	// Database Management Tool
	{
		Name:        "adminer",
		DisplayName: "Adminer",
		Category:    "database_tool",
		Description: "Modern, lightweight web GUI to inspect, query, and manage MariaDB/MySQL & PostgreSQL databases.",
		CheckPaths:  []string{"/usr/share/adminer/adminer.php", "/usr/share/adminer/index.php", "/usr/share/adminer"},
		AptPackages: []string{"adminer", "php-mysql", "php-pgsql", "php-sqlite3"},
		Ports:       "URI: /adminer/",
	},
}

type ServiceManager struct{}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

// IsInstalled checks if the service binaries or configuration directories exist on the host.
func (sm *ServiceManager) IsInstalled(serviceName string) bool {
	for _, def := range Catalog {
		if def.Name == serviceName {
			for _, p := range def.CheckPaths {
				if _, err := os.Stat(p); err == nil {
					return true
				}
			}
			break
		}
	}
	return false
}

// ExecuteSystemctl executes systemctl safely with strict whitelist validation and timeout.
func (sm *ServiceManager) ExecuteSystemctl(ctx context.Context, action, serviceName string) (string, error) {
	if !AllowedActions[action] {
		return "", fmt.Errorf("%w: %s", ErrDisallowedAction, action)
	}

	if !AllowedServices[serviceName] {
		return "", fmt.Errorf("%w: %s", ErrDisallowedService, serviceName)
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", action, serviceName)
	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))

	if err != nil {
		if action == "is-active" {
			return outStr, nil
		}
		// If service is apache2, capture apachectl -t for diagnostics
		extra := ""
		if serviceName == "apache2" {
			testCmd := exec.CommandContext(ctx, "apachectl", "-t")
			if tOut, _ := testCmd.CombinedOutput(); len(tOut) > 0 {
				extra = fmt.Sprintf(" [apachectl: %s]", strings.TrimSpace(string(tOut)))
			}
		}
		return outStr, fmt.Errorf("systemctl %s %s failed: %w (%s)%s", action, serviceName, err, outStr, extra)
	}

	return outStr, nil
}

func disabledFilePath(serviceName string) string {
	return filepath.Join("/var/lib/seintech/services", serviceName+".disabled")
}

func (sm *ServiceManager) isServiceDisabled(serviceName string) bool {
	_, err := os.Stat(disabledFilePath(serviceName))
	return err == nil
}

// GetStatus checks if the service is currently running.
func (sm *ServiceManager) GetStatus(ctx context.Context, serviceName string) (ServiceState, error) {
	if serviceName == "adminer" {
		if !sm.IsInstalled(serviceName) {
			return ServiceState{
				Name:   serviceName,
				Status: "not_installed",
				Active: false,
			}, nil
		}
		if sm.isServiceDisabled(serviceName) {
			return ServiceState{
				Name:   serviceName,
				Status: "stopped",
				Active: false,
			}, nil
		}
		return ServiceState{
			Name:   serviceName,
			Status: "running",
			Active: true,
		}, nil
	}

	out, err := sm.ExecuteSystemctl(ctx, "is-active", serviceName)
	state := ServiceState{
		Name:   serviceName,
		Status: out,
		Active: out == "active",
	}
	return state, err
}

func (sm *ServiceManager) Start(ctx context.Context, serviceName string) error {
	if serviceName == "adminer" {
		_ = os.Remove(disabledFilePath(serviceName))
		sm.AutoConfigureAdminer(ctx)
		_, err := sm.ExecuteSystemctl(ctx, "reload", "nginx")
		return err
	}
	if serviceName == "apache2" {
		sm.ensureApachePorts(ctx)
	}
	_, err := sm.ExecuteSystemctl(ctx, "start", serviceName)
	return err
}

func (sm *ServiceManager) Stop(ctx context.Context, serviceName string) error {
	if serviceName == "adminer" {
		_ = os.MkdirAll("/var/lib/seintech/services", 0755)
		_ = os.WriteFile(disabledFilePath(serviceName), []byte("stopped"), 0644)
		_, err := sm.ExecuteSystemctl(ctx, "reload", "nginx")
		return err
	}
	_, err := sm.ExecuteSystemctl(ctx, "stop", serviceName)
	return err
}

func (sm *ServiceManager) Restart(ctx context.Context, serviceName string) error {
	if serviceName == "adminer" {
		_ = os.Remove(disabledFilePath(serviceName))
		sm.AutoConfigureAdminer(ctx)
		_, err := sm.ExecuteSystemctl(ctx, "reload", "nginx")
		return err
	}
	if serviceName == "apache2" {
		sm.ensureApachePorts(ctx)
	}
	_, err := sm.ExecuteSystemctl(ctx, "restart", serviceName)
	return err
}

func (sm *ServiceManager) Reload(ctx context.Context, serviceName string) error {
	if serviceName == "adminer" {
		_, err := sm.ExecuteSystemctl(ctx, "reload", "nginx")
		return err
	}
	_, err := sm.ExecuteSystemctl(ctx, "reload", serviceName)
	return err
}

// GetAllServicesList returns status for all cataloged services.
func (sm *ServiceManager) GetAllServicesList(ctx context.Context) []ServiceDetail {
	var list []ServiceDetail

	for _, def := range Catalog {
		installed := sm.IsInstalled(def.Name)
		active := false
		status := "not_installed"

		if installed {
			st, err := sm.GetStatus(ctx, def.Name)
			if err == nil && st.Active {
				active = true
				status = "running"
			} else {
				status = "stopped"
			}
		}

		list = append(list, ServiceDetail{
			Name:        def.Name,
			DisplayName: def.DisplayName,
			Category:    def.Category,
			Description: def.Description,
			Installed:   installed,
			Active:      active,
			Status:      status,
			Packages:    strings.Join(def.AptPackages, ", "),
			Ports:       def.Ports,
		})
	}

	return list
}

// InstallService installs the package on Debian/Ubuntu systems via apt-get.
func (sm *ServiceManager) InstallService(ctx context.Context, serviceName string) error {
	var targetDef *ServiceDefinition
	for i := range Catalog {
		if Catalog[i].Name == serviceName {
			targetDef = &Catalog[i]
			break
		}
	}

	if targetDef == nil {
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	// 5-minute timeout for package installation
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Special setup for PHP versions on Debian: ensure Sury repository is available
	if targetDef.Category == "php" {
		sm.ensureSuryPHPRepo(ctx)
	}
	// Web server port pre-configurations before installation starts
	if serviceName == "apache2" {
		sm.ensureApachePorts(ctx)
	}

	// Always preseed debconf so NO interactive prompts ever block APT or terminal execution
	sm.PreseedDebconf(ctx)

	// Recover from any interrupted dpkg / debconf prompt
	recoverCmd := exec.CommandContext(ctx, "dpkg", "--configure", "-a")
	recoverCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = recoverCmd.Run()

	// apt-get update
	updateCmd := exec.CommandContext(ctx, "apt-get", "update", "-y")
	updateCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = updateCmd.Run()

	// apt-get install -y <packages> with force-confold so pre-configured files aren't overridden
	args := append([]string{"install", "-y", "-o", "Dpkg::Options::=--force-confold", "-o", "Dpkg::Options::=--force-confdef"}, targetDef.AptPackages...)
	installCmd := exec.CommandContext(ctx, "apt-get", args...)
	installCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := installCmd.CombinedOutput()
	if err != nil && serviceName == "adminer" {
		// Fallback for adminer if APT package is unavailable:
		// Download standalone official adminer.php directly
		_ = os.MkdirAll("/usr/share/adminer", 0755)
		fbScript := `
			export DEBIAN_FRONTEND=noninteractive
			apt-get install -y curl wget php-mysql php-pgsql php-sqlite3 php-mbstring || true
			mkdir -p /usr/share/adminer
			curl -fsSL https://github.com/vrana/adminer/releases/download/v4.8.1/adminer-4.8.1.php -o /usr/share/adminer/adminer.php || \
			wget -qO /usr/share/adminer/adminer.php https://github.com/vrana/adminer/releases/download/v4.8.1/adminer-4.8.1.php || \
			curl -fsSL https://www.adminer.org/latest.php -o /usr/share/adminer/adminer.php || true
			chown -R www-data:www-data /usr/share/adminer
		`
		_ = exec.CommandContext(ctx, "bash", "-c", fbScript).Run()
		if sm.IsInstalled("adminer") {
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("package installation failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Post-install port and configuration enforcement
	if serviceName == "apache2" {
		sm.ensureApachePorts(ctx)
	}
	if serviceName == "adminer" {
		_ = os.Remove(disabledFilePath("adminer"))
		sm.AutoConfigureAdminer(ctx)
	}

	// Enable and start the installed service
	if targetDef.Category != "database_tool" {
		_, _ = sm.ExecuteSystemctl(ctx, "enable", targetDef.Name)
		_, _ = sm.ExecuteSystemctl(ctx, "start", targetDef.Name)
	} else {
		_, _ = sm.ExecuteSystemctl(ctx, "reload", "nginx")
	}

	return nil
}

// UninstallService removes a service package cleanly.
func (sm *ServiceManager) UninstallService(ctx context.Context, serviceName string) error {
	var targetDef *ServiceDefinition
	for i := range Catalog {
		if Catalog[i].Name == serviceName {
			targetDef = &Catalog[i]
			break
		}
	}

	if targetDef == nil {
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Preseed debconf to avoid any interactive prompt during removal (e.g. dbconfig purge prompts)
	sm.PreseedDebconf(ctx)

	// Recover any locked/interrupted dpkg states
	recoverCmd := exec.CommandContext(ctx, "dpkg", "--configure", "-a")
	recoverCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = recoverCmd.Run()

	// Stop & disable service first (for daemon services)
	if targetDef.Category != "database_tool" {
		_ = sm.Stop(ctx, targetDef.Name)
		_, _ = sm.ExecuteSystemctl(ctx, "disable", targetDef.Name)
	}

	// 1. Try batch purge
	args := append([]string{"purge", "-y", "--allow-change-held-packages"}, targetDef.AptPackages...)
	purgeCmd := exec.CommandContext(ctx, "apt-get", args...)
	purgeCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = purgeCmd.Run()

	// 2. Fallback per-package purge in case one package in the list was not found
	for _, pkg := range targetDef.AptPackages {
		pCmd := exec.CommandContext(ctx, "apt-get", "purge", "-y", pkg)
		pCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		_ = pCmd.Run()
	}

	// 3. Autoremove unused dependencies
	autoCmd := exec.CommandContext(ctx, "apt-get", "autoremove", "-y", "--purge")
	autoCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = autoCmd.Run()

	// 4. Forcefully remove any residual check paths and custom files left behind
	for _, p := range targetDef.CheckPaths {
		_ = os.RemoveAll(p)
	}

	if serviceName == "adminer" {
		_ = os.RemoveAll("/usr/share/adminer")
		_ = os.RemoveAll("/etc/adminer")
	} else if serviceName == "phpmyadmin" {
		_ = os.RemoveAll("/usr/share/phpmyadmin")
		_ = os.RemoveAll("/etc/phpmyadmin")
		_ = os.RemoveAll("/var/lib/phpmyadmin")
		_ = os.RemoveAll("/var/lib/seintech/pma_tokens")
		_ = os.Remove("/etc/nginx/snippets/phpmyadmin.conf")
	} else if serviceName == "phppgadmin" {
		_ = os.RemoveAll("/usr/share/phppgadmin")
		_ = os.RemoveAll("/etc/phppgadmin")
		_ = os.Remove("/etc/nginx/snippets/phppgadmin.conf")
	} else if strings.HasPrefix(serviceName, "php") && strings.HasSuffix(serviceName, "-fpm") {
		ver := strings.TrimPrefix(strings.TrimSuffix(serviceName, "-fpm"), "php")
		_ = os.RemoveAll(filepath.Join("/etc/php", ver))
		_ = os.Remove(fmt.Sprintf("/run/php/php%s-fpm.sock", ver))
		_ = os.Remove(fmt.Sprintf("/run/php/php%s-fpm.pid", ver))
		_ = os.Remove(fmt.Sprintf("/usr/sbin/php-fpm%s", ver))
		_ = os.Remove(fmt.Sprintf("/usr/bin/php%s", ver))
	}

	_ = os.Remove(disabledFilePath(serviceName))

	// Reset systemd failed state & reload daemon
	_ = exec.CommandContext(ctx, "systemctl", "reset-failed", targetDef.Name).Run()
	_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()

	if targetDef.Category == "database_tool" || targetDef.Category == "webserver" || targetDef.Category == "php" {
		_, _ = sm.ExecuteSystemctl(ctx, "reload", "nginx")
	}

	return nil
}

// PreseedDebconf sets debconf selections non-interactively to prevent any interactive dialogs.
func (sm *ServiceManager) PreseedDebconf(ctx context.Context) {
	preseeds := []string{
		"phpmyadmin phpmyadmin/dbconfig-install boolean false",
		"phpmyadmin phpmyadmin/dbconfig-reinstall boolean false",
		"phpmyadmin phpmyadmin/dbconfig-remove boolean false",
		"phpmyadmin phpmyadmin/purge boolean false",
		"phpmyadmin phpmyadmin/reconfigure-webserver multiselect none",
		"phpmyadmin phpmyadmin/app-password-confirm password",
		"phpmyadmin phpmyadmin/mysql/admin-pass password",
		"phpmyadmin phpmyadmin/mysql/app-pass password",
		"phpmyadmin phpmyadmin/mysql/method select Unix socket",
		"phpmyadmin phpmyadmin/setup-username string root",
		"phpmyadmin phpmyadmin/setup-password password",
		"phpmyadmin phpmyadmin/internal/skip-preseed boolean true",
		"dbconfig-common dbconfig-common/dbconfig-install boolean false",
		"dbconfig-common dbconfig-common/dbconfig-reinstall boolean false",
		"dbconfig-common dbconfig-common/dbconfig-upgrade boolean false",
		"dbconfig-common dbconfig-common/dbconfig-remove boolean false",
		"dbconfig-common dbconfig-common/purge boolean false",
		"phppgadmin phppgadmin/reconfigure-webserver multiselect none",
		"phppgadmin phppgadmin/dbconfig-install boolean false",
		"phppgadmin phppgadmin/dbconfig-remove boolean false",
		"phppgadmin phppgadmin/purge boolean false",
	}

	for _, ps := range preseeds {
		cmd := exec.CommandContext(ctx, "debconf-set-selections")
		cmd.Stdin = strings.NewReader(ps + "\n")
		_ = cmd.Run()
	}
}

// AutoConfigurePhpMyAdmin creates blowfish secret, fixes permissions, and configures phpMyAdmin SSO via SignonScript.
func (sm *ServiceManager) AutoConfigurePhpMyAdmin(ctx context.Context) {
	if !sm.IsInstalled("phpmyadmin") && func() bool { _, err := os.Stat("/usr/share/phpmyadmin"); return err != nil }() {
		return
	}

	// 1. Ensure token exchange directory exists with open write permissions
	_ = os.MkdirAll("/var/lib/seintech/pma_tokens", 0777)
	_ = os.Chmod("/var/lib/seintech/pma_tokens", 0777)

	// 2. Ensure blowfish secret file exists and is valid
	secretFile := "/var/lib/phpmyadmin/blowfish_secret.inc.php"
	if fi, err := os.Stat(secretFile); err != nil || fi.Size() < 20 {
		_ = os.MkdirAll("/var/lib/phpmyadmin", 0755)
		bytes := make([]byte, 16)
		_, _ = rand.Read(bytes)
		secret := fmt.Sprintf("%x", bytes) // 32 chars hex
		content := fmt.Sprintf("<?php\n$cfg['blowfish_secret'] = '%s';\n", secret)
		_ = os.WriteFile(secretFile, []byte(content), 0644)
		_ = exec.CommandContext(ctx, "chown", "-R", "www-data:www-data", "/var/lib/phpmyadmin").Run()
	}

	// 3. Ensure tmp and session directories exist for caching/sessions
	_ = os.MkdirAll("/var/lib/phpmyadmin/tmp", 0777)
	_ = os.Chmod("/var/lib/phpmyadmin/tmp", 0777)
	_ = os.MkdirAll("/usr/share/phpmyadmin/tmp", 0777)
	_ = os.Chmod("/usr/share/phpmyadmin/tmp", 0777)
	_ = os.MkdirAll("/var/lib/php/sessions", 0777)
	_ = os.Chmod("/var/lib/php/sessions", 0777)
	_ = os.MkdirAll("/etc/phpmyadmin/conf.d", 0755)

	// 4. Clean up deprecated 01-seintech.inc.php if present
	_ = os.Remove("/etc/phpmyadmin/conf.d/01-seintech.inc.php")

	// 5. Configure master phpMyAdmin configuration (supporting standard Cookie & SignonScript SSO)
	customConf := `<?php
/**
 * Seintech Panel - phpMyAdmin Master Configuration
 */
declare(strict_types=1);

// 1. Secret key for cookie encryption (32+ bytes)
$cfg['blowfish_secret'] = 's31nt3chP4n3lS3cur3Bl0wf1shS3cr3tK3y!2026';
if (file_exists('/var/lib/phpmyadmin/blowfish_secret.inc.php')) {
    @include '/var/lib/phpmyadmin/blowfish_secret.inc.php';
}

// 2. Temp and cache directories for Twig template engine & session caching
$cfg['TempDir'] = '/var/lib/phpmyadmin/tmp';
if (!is_dir($cfg['TempDir']) || !is_writable($cfg['TempDir'])) {
    $cfg['TempDir'] = '/usr/share/phpmyadmin/tmp';
}

// 3. MySQL / MariaDB connection detection
$sock = '';
foreach (['/run/mysqld/mysqld.sock', '/var/run/mysqld/mysqld.sock', '/tmp/mysql.sock'] as $candidate) {
    if (file_exists($candidate)) {
        $sock = $candidate;
        break;
    }
}

// Include Debian / custom conf.d overrides first if present
if (is_dir('/etc/phpmyadmin/conf.d/')) {
    foreach (glob('/etc/phpmyadmin/conf.d/*.php') as $filename) {
        $base = basename($filename);
        if ($base !== '01-seintech.inc.php' && $base !== '99-seintech.inc.php') {
            @include $filename;
        }
    }
}

// Server 1: Seintech Panel Auto-Login (Single Sign-On via SignonScript) - Default
$cfg['Servers'][1]['auth_type'] = 'signon';
$cfg['Servers'][1]['SignonScript'] = '/usr/share/phpmyadmin/signon_auth.php';
$cfg['Servers'][1]['SignonURL'] = '/phpmyadmin/index.php?server=2';
$cfg['Servers'][1]['LogoutURL'] = '/databases';
$cfg['Servers'][1]['host'] = 'localhost';
if (!empty($sock)) {
    $cfg['Servers'][1]['socket'] = $sock;
    $cfg['Servers'][1]['connect_type'] = 'socket';
}
$cfg['Servers'][1]['compress'] = false;
$cfg['Servers'][1]['AllowNoPassword'] = false;
$cfg['Servers'][1]['verbose'] = 'Seintech Panel Auto-Login (SSO)';

// Server 2: Standard Cookie Authentication (Manual Login)
$cfg['Servers'][2]['auth_type'] = 'cookie';
$cfg['Servers'][2]['host'] = 'localhost';
if (!empty($sock)) {
    $cfg['Servers'][2]['socket'] = $sock;
    $cfg['Servers'][2]['connect_type'] = 'socket';
}
$cfg['Servers'][2]['compress'] = false;
$cfg['Servers'][2]['AllowNoPassword'] = false;
$cfg['Servers'][2]['verbose'] = 'MariaDB / MySQL (Manual Login)';

// General settings
$cfg['UploadDir'] = '';
$cfg['SaveDir'] = '';
$cfg['SendErrorReports'] = 'never';
$cfg['CheckConfigurationPermissions'] = false;
`
	// Write configuration to all standard locations
	usrConf := "/usr/share/phpmyadmin/config.inc.php"
	if fi, err := os.Lstat(usrConf); err == nil && (fi.Mode()&os.ModeSymlink != 0) {
		_ = os.Remove(usrConf)
	}
	_ = os.WriteFile(usrConf, []byte(customConf), 0644)
	_ = os.WriteFile("/etc/phpmyadmin/config.inc.php", []byte(customConf), 0644)
	_ = os.WriteFile("/etc/phpmyadmin/conf.d/99-seintech.inc.php", []byte(customConf), 0644)
	_ = exec.CommandContext(ctx, "chown", "www-data:www-data", usrConf).Run()

	// 6. Install SignonScript SSO authentication handler
	signonAuthCode := `<?php
/**
 * Seintech Panel SSO Authentication Handler for phpMyAdmin (SignonScript)
 */
declare(strict_types=1);

if (!function_exists('get_login_credentials')) {
    function get_login_credentials(string $user): array
    {
        // 1. Check if token is passed via GET or REQUEST
        $token = $_GET['token'] ?? $_REQUEST['token'] ?? '';
        if (is_string($token) && preg_match('/^[a-f0-9]{32,64}$/', $token)) {
            $tokenFile = '/var/lib/seintech/pma_tokens/' . $token . '.json';
            if (file_exists($tokenFile)) {
                $raw = @file_get_contents($tokenFile);
                $data = @json_decode($raw, true);
                if (is_array($data) && time() <= ($data['expires'] ?? 0)) {
                    $u = (string)($data['user'] ?? '');
                    $p = (string)($data['pass'] ?? '');
                    if ($u !== '') {
                        if (session_status() === PHP_SESSION_ACTIVE) {
                            $_SESSION['pma_sso_user'] = $u;
                            $_SESSION['pma_sso_pass'] = $p;
                            $_SESSION['pma_sso_token'] = $token;
                        }
                        @unlink($tokenFile);
                        return [$u, $p];
                    }
                }
                @unlink($tokenFile);
            }
        }

        // 2. Check if credentials already exist in phpMyAdmin active session
        if (session_status() === PHP_SESSION_ACTIVE) {
            $u = (string)($_SESSION['pma_sso_user'] ?? '');
            $p = (string)($_SESSION['pma_sso_pass'] ?? '');
            if ($u !== '') {
                return [$u, $p];
            }
        }

        return ['', ''];
    }
}
`
	_ = os.WriteFile("/usr/share/phpmyadmin/signon_auth.php", []byte(signonAuthCode), 0644)
	_ = os.WriteFile("/etc/phpmyadmin/signon_auth.php", []byte(signonAuthCode), 0644)
	_ = exec.CommandContext(ctx, "chown", "www-data:www-data", "/usr/share/phpmyadmin/signon_auth.php").Run()

	// 7. Install SSO bridge gateway in /usr/share/phpmyadmin/sso.php
	ssoScript := `/usr/share/phpmyadmin/sso.php`
	ssoContent := `<?php
// Seintech Panel SSO Gateway for phpMyAdmin
declare(strict_types=1);

$token = $_GET['token'] ?? '';
if (!is_string($token) || !preg_match('/^[a-f0-9]{32,64}$/', $token)) {
    header("Location: /databases?error=invalid_token");
    exit;
}

$tokenFile = "/var/lib/seintech/pma_tokens/" . $token . ".json";
if (!file_exists($tokenFile)) {
    header("Location: /databases?error=token_not_found");
    exit;
}

$raw = @file_get_contents($tokenFile);
$data = @json_decode($raw, true);
if (!$data || time() > ($data['expires'] ?? 0)) {
    @unlink($tokenFile);
    header("Location: /databases?error=token_expired");
    exit;
}

$db = (string)($data['db'] ?? '');

// Clear stale session cookies so phpMyAdmin starts a clean session for this SSO user
foreach ($_COOKIE as $name => $val) {
    if (stripos($name, 'pma') !== false || stripos($name, 'phpmyadmin') !== false || stripos($name, 'signon') !== false) {
        setcookie($name, '', time() - 3600, '/');
        setcookie($name, '', time() - 3600, '/phpmyadmin/');
    }
}

// Redirect into phpMyAdmin with Server 1 (SSO SignonScript) and target DB
$target = '/phpmyadmin/index.php?server=1&token=' . urlencode($token);
if (!empty($db)) {
    $target .= '&db=' . urlencode($db);
}

header("Location: " . $target);
exit;
`
	_ = os.WriteFile(ssoScript, []byte(ssoContent), 0644)
	_ = exec.CommandContext(ctx, "chown", "www-data:www-data", ssoScript).Run()
	_ = exec.CommandContext(ctx, "nginx", "-s", "reload").Run()
}

// AutoConfigurePhpPgAdmin adjusts security settings, installs php-pgsql, and installs SSO bridge script.
func (sm *ServiceManager) AutoConfigurePhpPgAdmin(ctx context.Context) {
	if !sm.IsInstalled("phppgadmin") {
		return
	}

	// 1. Ensure token exchange directory exists
	_ = os.MkdirAll("/var/lib/seintech/pma_tokens", 0777)
	_ = os.Chmod("/var/lib/seintech/pma_tokens", 0777)

	// 2. Ensure php-pgsql and php-mbstring extensions are installed for active PHP runtimes
	script := `
		export DEBIAN_FRONTEND=noninteractive
		need_reload=0
		# Detect active PHP version sockets
		for sock in /run/php/php*-fpm.sock; do
			if [ -e "$sock" ]; then
				ver=$(basename "$sock" | sed -E 's/php([0-9]+\.[0-9]+)-fpm\.sock/\1/')
				need_pkg=0
				for ext in pgsql mbstring; do
					if [ ! -f "/etc/php/$ver/fpm/conf.d/"*"$ext"*.ini ] && ! php$ver -m 2>/dev/null | grep -qi "^$ext$"; then
						need_pkg=1
						break
					fi
				done
				if [ "$need_pkg" -eq 1 ]; then
					apt-get update -y >/dev/null 2>&1 || true
					apt-get install -y php$ver-pgsql php$ver-mbstring || true
					phpenmod -v $ver pgsql pdo_pgsql mbstring 2>/dev/null || true
					need_reload=1
				fi
			fi
		done

		# Also ensure generic php-pgsql & php-mbstring packages
		if ! dpkg -s php-pgsql >/dev/null 2>&1 || ! dpkg -s php-mbstring >/dev/null 2>&1; then
			apt-get install -y php-pgsql php-mbstring || true
			phpenmod pgsql pdo_pgsql mbstring 2>/dev/null || true
			need_reload=1
		fi

		if [ "$need_reload" -eq 1 ]; then
			for svc in $(systemctl list-unit-files --type=service | grep -oE 'php[0-9\.]+-fpm\.service'); do
				systemctl restart "$svc" 2>/dev/null || true
			done
			systemctl reload nginx 2>/dev/null || true
		fi
	`
	_ = exec.CommandContext(ctx, "bash", "-c", script).Run()

	// 3. Adjust security settings and tuning in config.inc.php
	for _, confFile := range []string{"/etc/phppgadmin/config.inc.php", "/usr/share/phppgadmin/conf/config.inc.php"} {
		if data, err := os.ReadFile(confFile); err == nil {
			str := string(data)

			// Clean up any previously leaked tuning that was outside PHP tags
			if idx := strings.Index(str, "/* Seintech PHP8 Tuning */"); idx != -1 {
				str = str[:idx]
			}

			// Remove any closing PHP tag ?> from the file so everything remains strictly inside PHP
			str = strings.ReplaceAll(str, "?>", "")

			// Disable extra login security so all local users can log in
			str = strings.ReplaceAll(str, "$conf['extra_login_security'] = true;", "$conf['extra_login_security'] = false;")

			// Allow users to see all databases they have access to
			str = strings.ReplaceAll(str, "$conf['owned_only'] = true;", "$conf['owned_only'] = false;")

			// Ensure host is set to localhost
			str = strings.ReplaceAll(str, "$conf['servers'][0]['host'] = '';", "$conf['servers'][0]['host'] = 'localhost';")

			// Append PHP error suppression cleanly without modifying $conf['servers'] array keys
			tuning := "\n/* Seintech PHP8 Tuning */\n" +
				"@ini_set('display_errors', '0');\n" +
				"@error_reporting(E_ALL & ~E_DEPRECATED & ~E_STRICT & ~E_NOTICE & ~E_WARNING);\n"

			str = strings.TrimSpace(str) + "\n" + tuning
			_ = os.WriteFile(confFile, []byte(str), 0644)
		}
	}

	// 4. Install native SSO bridge script in /usr/share/phppgadmin/sso.php
	ssoScript := `/usr/share/phppgadmin/sso.php`
	ssoContent := `<?php
// Seintech Panel SSO Bridge for phpPgAdmin
$token = $_GET['token'] ?? '';
if (!preg_match('/^[a-f0-9]{32,64}$/', $token)) {
    die("Invalid token format");
}
$tokenFile = "/var/lib/seintech/pma_tokens/" . $token . ".json";
if (!file_exists($tokenFile)) {
    die("Token not found or already used");
}
$data = json_decode(file_get_contents($tokenFile), true);
@unlink($tokenFile);

if (!$data || time() > $data['expires']) {
    die("Token expired");
}

chdir('/usr/share/phppgadmin');

// Reset previous session data
session_name('PPA_ID');
@session_start();
$_SESSION = array();

// Read config to find exact configured server
$conf = array();
if (file_exists('/etc/phppgadmin/config.inc.php')) {
    include('/etc/phppgadmin/config.inc.php');
} elseif (file_exists('./conf/config.inc.php')) {
    include('./conf/config.inc.php');
}

$first_server = !empty($conf['servers']) ? reset($conf['servers']) : null;
if (!$first_server) {
    die("No PostgreSQL server configured in phpPgAdmin");
}

// In phpPgAdmin, server_id is strictly: host:port:sslmode
$server_id = $first_server['host'] . ':' . $first_server['port'] . ':' . $first_server['sslmode'];

// Set login request parameters before loading phpPgAdmin core library
$_POST['loginServer'] = $server_id;
$_POST['loginUsername'] = $data['user'];
$_POST['loginPassword_' . md5($server_id)] = $data['pass'];
$_POST['loginShared'] = '1';
$_REQUEST['server'] = $server_id;
if (!empty($data['db'])) {
    $_REQUEST['database'] = $data['db'];
}

$_no_db_connection = true;
require_once('./libraries/lib.inc.php');

// Explicitly register authenticated server credentials in session via Misc helper
if (isset($misc)) {
    $_server_info = $misc->getServerInfo($server_id);
    if (!empty($_server_info)) {
        $_server_info['username'] = $data['user'];
        $_server_info['password'] = $data['pass'];
        $misc->setServerInfo(null, $_server_info, $server_id);
    }
}
$_SESSION['sharedUsername'] = $data['user'];
$_SESSION['sharedPassword'] = $data['pass'];

session_write_close();

$db = urlencode($data['db'] ?? '');
if ($db !== '') {
    $detail = 'redirect.php?subject=database&server=' . urlencode($server_id) . '&database=' . $db;
} else {
    $detail = 'redirect.php?subject=server&server=' . urlencode($server_id);
}

// Redirect into frameset with the target detail page
header("Location: /phppgadmin/index.php?detail=" . urlencode($detail));
exit;
`
	_ = os.WriteFile(ssoScript, []byte(ssoContent), 0644)

	// 5. Enhance index.php to support ?detail= query parameter so SSO lands in the correct frameset view
	for _, indexPath := range []string{"/usr/share/phppgadmin/index.php"} {
		if data, err := os.ReadFile(indexPath); err == nil {
			str := string(data)
			if !strings.Contains(str, "detail_src") {
				oldMainframe := `$mainframe = '<frame src="intro.php" name="detail" id="detail" frameborder="0" />'`
				newMainframe := `$detail_src = (!empty($_GET['detail'])) ? $_GET['detail'] : 'intro.php';
	$mainframe = '<frame src="' . htmlspecialchars($detail_src) . '" name="detail" id="detail" frameborder="0" />'`
				if strings.Contains(str, oldMainframe) {
					str = strings.Replace(str, oldMainframe, newMainframe, 1)
					_ = os.WriteFile(indexPath, []byte(str), 0644)
				}
			}
		}
	}
	_ = exec.CommandContext(ctx, "nginx", "-s", "reload").Run()
}

func (sm *ServiceManager) ensurePhpMyAdminPrerequisites(ctx context.Context) {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return
	}
	// Enable Debian contrib & non-free repos so package phpmyadmin is discoverable in APT
	script := `
		export DEBIAN_FRONTEND=noninteractive
		for src in /etc/apt/sources.list.d/*.sources; do
			if [ -f "$src" ]; then
				sed -i 's/Components: main$/Components: main contrib non-free/g' "$src" 2>/dev/null || true
				sed -i 's/Components: main non-free-firmware$/Components: main contrib non-free non-free-firmware/g' "$src" 2>/dev/null || true
			fi
		done
		if [ -f /etc/apt/sources.list ]; then
			sed -i '/^deb / { /contrib/! s/main/main contrib/ }' /etc/apt/sources.list 2>/dev/null || true
		fi
	`
	_ = exec.CommandContext(ctx, "bash", "-c", script).Run()
}

// AutoConfigureAdminer installs dependencies, wrapper index.php with SSO bridge, and a modern CSS theme for Adminer.
func (sm *ServiceManager) AutoConfigureAdminer(ctx context.Context) {
	// 1. Ensure token exchange directory exists
	_ = os.MkdirAll("/var/lib/seintech/pma_tokens", 0777)
	_ = os.Chmod("/var/lib/seintech/pma_tokens", 0777)

	// 2. Ensure /usr/share/adminer exists
	_ = os.MkdirAll("/usr/share/adminer", 0755)

	// 3. Ensure PHP database driver extensions (mysql, pgsql, sqlite3, mbstring) and configure pg_hba.conf
	script := `
		export DEBIAN_FRONTEND=noninteractive
		
		# Install drivers for base system packages
		apt-get update -y >/dev/null 2>&1 || true
		apt-get install -y php-mysql php-pgsql php-sqlite3 php-mbstring curl wget 2>/dev/null || true
		phpenmod mysqli pdo_mysql pgsql pdo_pgsql mbstring 2>/dev/null || true

		# Also ensure drivers are enabled across all installed PHP version directories
		for phpdir in /etc/php/*; do
			if [ -d "$phpdir" ]; then
				ver=$(basename "$phpdir")
				apt-get install -y php$ver-mysql php$ver-pgsql php$ver-sqlite3 php$ver-mbstring 2>/dev/null || true
				phpenmod -v $ver mysqlnd mysqli pdo_mysql pgsql pdo_pgsql mbstring 2>/dev/null || true
			fi
		done

		# Ensure PostgreSQL allows local md5/scram password connections (avoid peer auth rejection on unix socket)
		for hba in /etc/postgresql/*/main/pg_hba.conf; do
			if [ -f "$hba" ]; then
				sed -i 's/local\s\+all\s\+all\s\+peer/local all all md5/g' "$hba" 2>/dev/null || true
			fi
		done
		systemctl reload postgresql 2>/dev/null || true

		# Restart PHP-FPM daemons so new extensions are immediately active
		for svc in $(systemctl list-unit-files --type=service | grep -oE 'php[0-9\.]+-fpm\.service'); do
			systemctl restart "$svc" 2>/dev/null || true
		done
	`
	_ = exec.CommandContext(ctx, "bash", "-c", script).Run()

	// 4. Install official standalone adminer.php from embedded assets if missing or corrupted
	adminerBin := "/usr/share/adminer/adminer.php"
	needInstallAdminer := true
	if fi, err := os.Stat(adminerBin); err == nil && fi.Size() > 50000 {
		needInstallAdminer = false
	}
	if needInstallAdminer {
		if embeddedAdminer, err := web.EmbeddedFiles.ReadFile("static/adminer.php"); err == nil && len(embeddedAdminer) > 50000 {
			_ = os.WriteFile(adminerBin, embeddedAdminer, 0644)
		} else {
			dlScript := `
				curl -fsSL https://github.com/vrana/adminer/releases/download/v4.8.1/adminer-4.8.1.php -o /usr/share/adminer/adminer.php || \
				wget -qO /usr/share/adminer/adminer.php https://github.com/vrana/adminer/releases/download/v4.8.1/adminer-4.8.1.php || \
				curl -fsSL https://www.adminer.org/latest.php -o /usr/share/adminer/adminer.php || true
			`
			_ = exec.CommandContext(ctx, "bash", "-c", dlScript).Run()
		}
	}

	// 5. Install Seintech SSO & Plugin Wrapper in /usr/share/adminer/index.php
	wrapperScript := `<?php
// Seintech Panel SSO & Modern Theme Wrapper for Adminer
declare(strict_types=1);

@ini_set('display_errors', '0');
@ini_set('log_errors', '1');

// 1. Process One-Time SSO Token from Seintech Panel
$token = $_GET['token'] ?? '';
if (is_string($token) && preg_match('/^[a-f0-9]{32,64}$/', $token)) {
    $tokenFile = '/var/lib/seintech/pma_tokens/' . $token . '.json';
    if (file_exists($tokenFile)) {
        $raw = @file_get_contents($tokenFile);
        $data = @json_decode($raw, true);
        @unlink($tokenFile);

        if ($data && time() <= ($data['expires'] ?? 0)) {
            $isPg = ($data['type'] ?? '') === 'postgres';
            $driver = $isPg ? 'pgsql' : 'server';
            $server = '127.0.0.1'; // 127.0.0.1 ensures TCP loopback authentication
            $username = (string)($data['user'] ?? '');
            $password = (string)($data['pass'] ?? '');
            $db = (string)($data['db'] ?? '');

            if (session_status() === PHP_SESSION_NONE) {
                session_name('adminer_sid');
                @session_start();
            }

            // Store credentials in session for Adminer credentials() hook
            $_SESSION['seintech_auth'] = [
                'driver'   => $driver,
                'server'   => $server,
                'username' => $username,
                'password' => $password,
                'db'       => $db,
                'time'     => time(),
            ];

            if (!isset($_SESSION['pwds'])) {
                $_SESSION['pwds'] = [];
            }
            $_SESSION['pwds'][$driver][$server][$username] = $password;

            session_write_close();

            $target = 'index.php?' . $driver . '=' . urlencode($server) . '&username=' . urlencode($username);
            if (!empty($db)) {
                $target .= '&db=' . urlencode($db);
            }
            header('Location: ' . $target);
            exit;
        }
    }
}

// 2. Set session name if session is not started yet
if (session_status() === PHP_SESSION_NONE) {
    session_name('adminer_sid');
}

// 3. Clear SSO credentials on explicit logout
if (isset($_GET['logout'])) {
    if (session_status() === PHP_SESSION_NONE) {
        session_name('adminer_sid');
        @session_start();
    }
    unset($_SESSION['seintech_auth']);
    unset($_SESSION['pwds']);
    session_write_close();
}

// 4. Custom Adminer object supporting both Adminer 4 and 5
function adminer_object() {
    if (class_exists('Adminer\\Adminer')) {
        if (!class_exists('SeintechAdminer5', false)) {
            class SeintechAdminer5 extends Adminer\Adminer {
                function name() {
                    return '<a href="/databases" class="seintech-brand" style="text-decoration:none;color:inherit;"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M4 7v10c0 2 3.5 3 8 3s8-1 8-3V7M4 7c0 2 3.5 3 8 3s8-1 8-3M4 7c0-2 3.5-3 8-3s8 1 8 3m0 5c0 2-3.5 3-8 3s-8-1-8-3"/></svg> Seintech Database Manager</a>';
                }
                function credentials() {
                    $auth = $_SESSION['seintech_auth'] ?? [];
                    if (!empty($auth['password'])) {
                        return [
                            $auth['server'] ?? '127.0.0.1',
                            $auth['username'] ?? '',
                            $auth['password'] ?? ''
                        ];
                    }
                    return array(SERVER, $_GET["username"] ?? "", get_password());
                }
                function database() {
                    if (!empty($_GET['db'])) {
                        return (string)$_GET['db'];
                    }
                    $auth = $_SESSION['seintech_auth'] ?? [];
                    if (!empty($auth['db'])) {
                        return (string)$auth['db'];
                    }
                    return parent::database();
                }
                function login($login, $password) {
                    return true;
                }
            }
        }
        return new SeintechAdminer5();
    }

    if (!class_exists('SeintechAdminer4', false)) {
        class SeintechAdminer4 extends Adminer {
            function name() {
                return '<a href="/databases" class="seintech-brand" style="text-decoration:none;color:inherit;"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M4 7v10c0 2 3.5 3 8 3s8-1 8-3V7M4 7c0 2 3.5 3 8 3s8-1 8-3M4 7c0-2 3.5-3 8-3s8 1 8 3m0 5c0 2-3.5 3-8 3s-8-1-8-3"/></svg> Seintech Database Manager</a>';
            }
            function credentials() {
                $auth = $_SESSION['seintech_auth'] ?? [];
                if (!empty($auth['password'])) {
                    return [
                        $auth['server'] ?? '127.0.0.1',
                        $auth['username'] ?? '',
                        $auth['password'] ?? ''
                    ];
                }
                return array(SERVER, $_GET["username"] ?? "", get_password());
            }
            function database() {
                if (!empty($_GET['db'])) {
                    return (string)$_GET['db'];
                }
                $auth = $_SESSION['seintech_auth'] ?? [];
                if (!empty($auth['db'])) {
                    return (string)$auth['db'];
                }
                return parent::database();
            }
            function login($login, $password) {
                return true;
            }
        }
    }
    return new SeintechAdminer4();
}

// 5. Include Adminer core
if (file_exists(__DIR__ . '/adminer.php')) {
    include __DIR__ . '/adminer.php';
} elseif (file_exists('/usr/share/adminer/adminer.php')) {
    include '/usr/share/adminer/adminer.php';
} else {
    echo 'Adminer core binary missing. Please reinstall Adminer from Seintech Panel.';
}
`
	_ = os.WriteFile("/usr/share/adminer/index.php", []byte(wrapperScript), 0644)
	_ = os.WriteFile("/usr/share/adminer/sso.php", []byte("<?php\nrequire_once __DIR__ . '/index.php';\n"), 0644)

	// 6. Install Modern CSS Theme for Adminer
	modernTheme := `/* Seintech Modern Theme for Adminer */
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

:root {
    --primary: #4f46e5;
    --primary-hover: #4338ca;
    --primary-light: #eef2ff;
    --bg-page: #f8fafc;
    --bg-surface: #ffffff;
    --border: #e2e8f0;
    --border-hover: #cbd5e1;
    --text-primary: #0f172a;
    --text-secondary: #475569;
    --text-muted: #64748b;
    --radius-sm: 6px;
    --radius-md: 10px;
    --radius-lg: 14px;
    --shadow-sm: 0 1px 2px rgba(0,0,0,0.05);
    --shadow-md: 0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05);
    --font-sans: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

* { box-sizing: border-box; }

html, body {
    margin: 0;
    padding: 0;
    font-family: var(--font-sans);
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-primary);
    background-color: var(--bg-page);
    -webkit-font-smoothing: antialiased;
}

a {
    color: var(--primary);
    text-decoration: none;
    transition: color 0.15s ease;
}
a:hover {
    color: var(--primary-hover);
    text-decoration: underline;
}

#menu {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    width: 260px;
    background: var(--bg-surface);
    border-right: 1px solid var(--border);
    padding: 20px 16px;
    overflow-y: auto;
    z-index: 100;
    box-shadow: 1px 0 3px rgba(0,0,0,0.03);
}

#menu h1 {
    font-size: 15px;
    font-weight: 700;
    margin: 0 0 18px 0;
    padding-bottom: 14px;
    border-bottom: 1px solid var(--border);
}

.seintech-brand {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: var(--primary);
    font-weight: 700;
    letter-spacing: -0.01em;
}

#menu .version {
    font-size: 11px;
    color: var(--text-muted);
    font-weight: normal;
}

#menu p {
    margin: 6px 0;
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    transition: background-color 0.15s ease;
}
#menu p:hover {
    background: var(--primary-light);
}

#tables {
    list-style: none;
    padding: 0;
    margin: 10px 0;
}
#tables li {
    margin-bottom: 2px;
}
#tables a {
    display: block;
    padding: 5px 8px;
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    font-size: 12.5px;
    transition: all 0.15s ease;
}
#tables a:hover, #tables .active {
    background: var(--primary-light);
    color: var(--primary);
    font-weight: 500;
    text-decoration: none;
}

#content {
    margin-left: 260px;
    padding: 28px 36px;
    max-width: 1400px;
}

h2 {
    font-size: 20px;
    font-weight: 700;
    color: var(--text-primary);
    margin: 0 0 18px 0;
    display: flex;
    align-items: center;
    gap: 8px;
    letter-spacing: -0.02em;
}

#breadcrumb {
    background: transparent;
    padding: 0 0 14px 0;
    color: var(--text-muted);
    font-size: 12.5px;
}
#breadcrumb a {
    color: var(--text-secondary);
    font-weight: 500;
}

table {
    border-collapse: separate;
    border-spacing: 0;
    width: 100%;
    margin: 16px 0 24px 0;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-sm);
    overflow: hidden;
}

thead th, th {
    background: #f1f5f9;
    color: var(--text-secondary);
    font-size: 11.5px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    text-align: left;
}

td {
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 13px;
}

tbody tr:last-child td {
    border-bottom: none;
}

tbody tr:hover td {
    background-color: #f8fafc;
}

input[type="text"], input[type="password"], input[type="number"], select, textarea {
    font-family: var(--font-sans);
    font-size: 13px;
    color: var(--text-primary);
    background: #ffffff;
    border: 1px solid var(--border-hover);
    border-radius: var(--radius-sm);
    padding: 7px 12px;
    outline: none;
    transition: all 0.15s ease;
}

input[type="text"]:focus, input[type="password"]:focus, select:focus, textarea:focus {
    border-color: var(--primary);
    box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.12);
}

textarea {
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 1.45;
    background: #ffffff;
    border-radius: var(--radius-md);
    padding: 12px;
}

input[type="submit"], button, .button {
    font-family: var(--font-sans);
    font-size: 13px;
    font-weight: 600;
    padding: 7px 16px;
    color: #ffffff;
    background: var(--primary);
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    transition: all 0.15s ease;
    box-shadow: var(--shadow-sm);
}

input[type="submit"]:hover, button:hover, .button:hover {
    background: var(--primary-hover);
    box-shadow: var(--shadow-md);
    text-decoration: none;
}

.message {
    background: #ecfdf5;
    border: 1px solid #a7f3d0;
    color: #065f46;
    border-radius: var(--radius-md);
    padding: 12px 16px;
    font-size: 13px;
    font-weight: 500;
    margin: 16px 0;
    box-shadow: var(--shadow-sm);
}

.error {
    background: #fff1f2;
    border: 1px solid #fecdd3;
    color: #9f1239;
    border-radius: var(--radius-md);
    padding: 12px 16px;
    font-size: 13px;
    font-weight: 500;
    margin: 16px 0;
    box-shadow: var(--shadow-sm);
}

fieldset {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 16px 20px;
    margin: 16px 0;
    box-shadow: var(--shadow-sm);
}

legend {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
    padding: 0 8px;
}
`
	_ = os.WriteFile("/usr/share/adminer/adminer.css", []byte(modernTheme), 0644)
	_ = exec.CommandContext(ctx, "chown", "-R", "www-data:www-data", "/usr/share/adminer").Run()
	_ = exec.CommandContext(ctx, "nginx", "-s", "reload").Run()
}

// EnsureDatabaseTools preseeds debconf and auto-configures database tools if present.
func (sm *ServiceManager) EnsureDatabaseTools(ctx context.Context) {
	sm.PreseedDebconf(ctx)
	sm.AutoConfigureAdminer(ctx)
}

// EnsureSystemRepositories automatically configures official APT repositories for PHP (Sury/PPA) and database tools.
func (sm *ServiceManager) EnsureSystemRepositories(ctx context.Context) {
	sm.ensureSuryPHPRepo(ctx)
	sm.EnsureDatabaseTools(ctx)
}

func (sm *ServiceManager) ensureSuryPHPRepo(ctx context.Context) {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return
	}

	// Auto-detect Debian vs Ubuntu and distribution codename (trixie, bookworm, bullseye, noble, jammy, focal)
	script := `
		export DEBIAN_FRONTEND=noninteractive
		OS_ID="debian"
		CODENAME=""

		if [ -f /etc/os-release ]; then
			. /etc/os-release
			OS_ID="${ID:-debian}"
			CODENAME="${VERSION_CODENAME:-}"
		fi

		if [ -z "$CODENAME" ] && command -v lsb_release >/dev/null 2>&1; then
			CODENAME=$(lsb_release -sc)
		fi

		if [ -z "$CODENAME" ] && [ -n "$VERSION_ID" ]; then
			case "$VERSION_ID" in
				13*) CODENAME="trixie" ;;
				12*) CODENAME="bookworm" ;;
				11*) CODENAME="bullseye" ;;
				24.04*) CODENAME="noble" ;;
				22.04*) CODENAME="jammy" ;;
				20.04*) CODENAME="focal" ;;
			esac
		fi

		if [ -z "$CODENAME" ]; then
			if [ "$OS_ID" = "ubuntu" ]; then
				CODENAME="noble"
			else
				CODENAME="trixie"
			fi
		fi

		apt-get update -y
		apt-get install -y ca-certificates curl gnupg lsb-release wget software-properties-common || true

		mkdir -p /usr/share/keyrings /etc/apt/trusted.gpg.d /etc/apt/sources.list.d

		if [ "$OS_ID" = "ubuntu" ]; then
			if command -v add-apt-repository >/dev/null 2>&1; then
				add-apt-repository -y ppa:ondrej/php
			else
				curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x14AA40EC0831756756D7F66C4F4EA0AAE5267A6C" | gpg --dearmor -o /etc/apt/trusted.gpg.d/ondrej-php.gpg 2>/dev/null || true
				echo "deb [signed-by=/etc/apt/trusted.gpg.d/ondrej-php.gpg] https://ppa.launchpadcontent.net/ondrej/php/ubuntu $CODENAME main" > /etc/apt/sources.list.d/php.list
			fi
		else
			curl -fsSL https://packages.sury.org/php/apt.gpg -o /usr/share/keyrings/deb.sury.org-php.gpg 2>/dev/null || true
			cp /usr/share/keyrings/deb.sury.org-php.gpg /etc/apt/trusted.gpg.d/php.gpg 2>/dev/null || true
			echo "deb [signed-by=/usr/share/keyrings/deb.sury.org-php.gpg] https://packages.sury.org/php/ $CODENAME main" > /etc/apt/sources.list.d/php.list
		fi
		apt-get update -y
	`

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = cmd.Run()
}

func (sm *ServiceManager) ensureCaddyRepo(ctx context.Context) {
	if _, err := os.Stat("/etc/apt/sources.list.d/caddy-stable.list"); err == nil {
		return
	}
	script := "curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && " +
		"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list"
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = cmd.Run()
}

func (sm *ServiceManager) ensureApachePorts(ctx context.Context) {
	_ = os.MkdirAll("/etc/apache2", 0755)
	_ = os.MkdirAll("/etc/apache2/sites-available", 0755)
	_ = os.MkdirAll("/etc/apache2/sites-enabled", 0755)
	_ = os.MkdirAll("/etc/apache2/conf-available", 0755)
	_ = os.MkdirAll("/etc/apache2/conf-enabled", 0755)

	// Single Listen directive to avoid address binding conflict
	portsConf := `# Configured by Seintech Panel for Nginx Reverse Proxy Backend
Listen 8080
`
	_ = os.WriteFile("/etc/apache2/ports.conf", []byte(portsConf), 0644)

	// Global ServerName to avoid AH00558 domain name warning
	_ = os.WriteFile("/etc/apache2/conf-available/servername.conf", []byte("ServerName localhost\n"), 0644)

	// RemoteIP configuration so Apache and PHP ($_SERVER['REMOTE_ADDR']) get real visitor IP from Nginx
	remoteIPConf := `# Configured by Seintech Panel for Nginx Reverse Proxy
<IfModule mod_remoteip.c>
	RemoteIPHeader X-Forwarded-For
	RemoteIPInternalProxy 127.0.0.1
	RemoteIPInternalProxy ::1
</IfModule>
`
	_ = os.WriteFile("/etc/apache2/conf-available/remoteip.conf", []byte(remoteIPConf), 0644)

	// Remove default Debian conflicting sites from sites-enabled
	_ = os.Remove("/etc/apache2/sites-enabled/000-default.conf")
	_ = os.Remove("/etc/apache2/sites-enabled/default-ssl.conf")

	// Clean up / heal any sites-available files that had broken tags or port 80
	if entries, err := os.ReadDir("/etc/apache2/sites-available"); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".conf") {
				p := filepath.Join("/etc/apache2/sites-available", e.Name())
				if data, err := os.ReadFile(p); err == nil {
					s := string(data)
					if strings.Contains(s, "<VirtualHost 127.0.0.1:8080") || strings.Contains(s, "<VirtualHost *:80>") {
						s = strings.ReplaceAll(s, "<VirtualHost 127.0.0.1:8080 [::1]:8080 *:8080>", "<VirtualHost *:8080>")
						s = strings.ReplaceAll(s, "<VirtualHost 127.0.0.1:8080>", "<VirtualHost *:8080>")
						s = strings.ReplaceAll(s, "<VirtualHost *:80>", "<VirtualHost *:8080>")
						_ = os.WriteFile(p, []byte(s), 0644)
					}
				}
			}
		}
	}

	// Enable required Apache modules
	if _, err := exec.LookPath("a2enmod"); err == nil {
		cmd := exec.CommandContext(ctx, "a2enmod", "rewrite", "ssl", "proxy", "proxy_fcgi", "headers", "remoteip", "setenvif")
		_ = cmd.Run()
	}
	if _, err := exec.LookPath("a2enconf"); err == nil {
		cmdConf := exec.CommandContext(ctx, "a2enconf", "remoteip", "servername")
		_ = cmdConf.Run()
	}
	if _, err := exec.LookPath("a2dissite"); err == nil {
		cmdDis := exec.CommandContext(ctx, "a2dissite", "000-default", "default-ssl")
		_ = cmdDis.Run()
	}
}

// EnsureApacheConfiguration exposes Apache port & module setup for startup
func (sm *ServiceManager) EnsureApacheConfiguration(ctx context.Context) {
	if sm.IsInstalled("apache2") {
		sm.ensureApachePorts(ctx)
	}
}

// EnsureNginxConfiguration configures global upload limits, timeouts, and buffers for Nginx
func (sm *ServiceManager) EnsureNginxConfiguration(ctx context.Context) {
	if !sm.IsInstalled("nginx") {
		return
	}

	confDir := "/etc/nginx/conf.d"
	if _, err := os.Stat(confDir); os.IsNotExist(err) {
		_ = os.MkdirAll(confDir, 0755)
	}

	tuningFile := filepath.Join(confDir, "seintech_tuning.conf")
	tuningContent := `# Auto-generated by Seintech Panel
# Increases maximum upload file size, timeouts, and disables request buffering for large uploads
client_max_body_size 2048M;
client_body_buffer_size 128M;
proxy_read_timeout 600s;
proxy_connect_timeout 60s;
proxy_send_timeout 600s;
proxy_request_buffering off;
fastcgi_read_timeout 600s;
`
	needReload := false
	current, err := os.ReadFile(tuningFile)
	if err != nil || string(current) != tuningContent {
		_ = os.WriteFile(tuningFile, []byte(tuningContent), 0644)
		needReload = true
	}

	if needReload {
		if exec.CommandContext(ctx, "nginx", "-t").Run() == nil {
			_ = exec.CommandContext(ctx, "nginx", "-s", "reload").Run()
		}
	}
}

// CleanupDeprecatedRepositories removes deprecated APT repositories and signing keys (Caddy and OpenLiteSpeed)
func (sm *ServiceManager) CleanupDeprecatedRepositories(ctx context.Context) {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return
	}

	removed := false
	files := []string{
		"/etc/apt/sources.list.d/lst_debian_repo.list",
		"/etc/apt/sources.list.d/caddy-stable.list",
		"/etc/apt/trusted.gpg.d/lst_debian_repo.gpg",
		"/etc/apt/trusted.gpg.d/lst_repo.gpg",
		"/usr/share/keyrings/caddy-stable-archive-keyring.gpg",
	}

	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			_ = os.Remove(f)
			removed = true
		}
	}

	if removed {
		cmd := exec.CommandContext(ctx, "apt-get", "update", "-y")
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		_ = cmd.Run()
	}
}
