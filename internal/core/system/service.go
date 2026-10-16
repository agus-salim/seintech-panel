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
	"phpmyadmin":  true,
	"phppgadmin":  true,
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

	// Database Management Tools
	{
		Name:        "phpmyadmin",
		DisplayName: "phpMyAdmin",
		Category:    "database_tool",
		Description: "Web-based administration interface for MySQL and MariaDB databases.",
		CheckPaths:  []string{"/usr/share/phpmyadmin"},
		AptPackages: []string{"phpmyadmin"},
		Ports:       "URI: /phpmyadmin/",
	},
	{
		Name:        "phppgadmin",
		DisplayName: "phpPgAdmin",
		Category:    "database_tool",
		Description: "Web-based administration interface for PostgreSQL databases.",
		CheckPaths:  []string{"/usr/share/phppgadmin"},
		AptPackages: []string{"phppgadmin", "php-pgsql"},
		Ports:       "URI: /phppgadmin/",
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
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
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
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
		_ = os.Remove(disabledFilePath(serviceName))
		if serviceName == "phpmyadmin" {
			sm.AutoConfigurePhpMyAdmin(ctx)
		} else {
			sm.AutoConfigurePhpPgAdmin(ctx)
		}
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
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
		_ = os.MkdirAll("/var/lib/seintech/services", 0755)
		_ = os.WriteFile(disabledFilePath(serviceName), []byte("stopped"), 0644)
		_, err := sm.ExecuteSystemctl(ctx, "reload", "nginx")
		return err
	}
	_, err := sm.ExecuteSystemctl(ctx, "stop", serviceName)
	return err
}

func (sm *ServiceManager) Restart(ctx context.Context, serviceName string) error {
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
		_ = os.Remove(disabledFilePath(serviceName))
		if serviceName == "phpmyadmin" {
			sm.AutoConfigurePhpMyAdmin(ctx)
		} else {
			sm.AutoConfigurePhpPgAdmin(ctx)
		}
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
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
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
	if serviceName == "phpmyadmin" {
		sm.ensurePhpMyAdminPrerequisites(ctx)
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
	if err != nil && serviceName == "phpmyadmin" {
		// Fallback for phpmyadmin if APT package is unavailable in Debian testing/trixie:
		// Download official standalone release archive directly
		fbScript := `
			export DEBIAN_FRONTEND=noninteractive
			apt-get install -y curl wget tar php-mysql php-mbstring php-xml php-curl php-zip php-gd || true
			mkdir -p /usr/share/phpmyadmin /var/lib/phpmyadmin/tmp /etc/phpmyadmin/conf.d
			curl -fsSL https://www.phpmyadmin.net/downloads/phpMyAdmin-latest-all-languages.tar.gz -o /tmp/pma.tar.gz || \
			wget -qO /tmp/pma.tar.gz https://files.phpmyadmin.net/phpMyAdmin/5.2.2/phpMyAdmin-5.2.2-all-languages.tar.gz || true
			if [ -f "/tmp/pma.tar.gz" ] && [ -s "/tmp/pma.tar.gz" ]; then
				tar -xzf /tmp/pma.tar.gz -C /usr/share/phpmyadmin --strip-components=1
				rm -f /tmp/pma.tar.gz
				chown -R www-data:www-data /usr/share/phpmyadmin
			fi
		`
		_ = exec.CommandContext(ctx, "bash", "-c", fbScript).Run()
		if sm.IsInstalled("phpmyadmin") {
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
	if serviceName == "phpmyadmin" {
		_ = os.Remove(disabledFilePath("phpmyadmin"))
		sm.AutoConfigurePhpMyAdmin(ctx)
	}
	if serviceName == "phppgadmin" {
		_ = os.Remove(disabledFilePath("phppgadmin"))
		sm.AutoConfigurePhpPgAdmin(ctx)
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

	if serviceName == "phpmyadmin" {
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

// AutoConfigurePhpMyAdmin creates blowfish secret, fixes permissions, and configures phpMyAdmin SSO.
func (sm *ServiceManager) AutoConfigurePhpMyAdmin(ctx context.Context) {
	if !sm.IsInstalled("phpmyadmin") {
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

	// 3. Ensure tmp directory exists for caching/sessions
	tmpDir := "/var/lib/phpmyadmin/tmp"
	_ = os.MkdirAll(tmpDir, 0777)
	_ = os.Chmod(tmpDir, 0777)
	_ = exec.CommandContext(ctx, "chown", "-R", "www-data:www-data", "/var/lib/phpmyadmin").Run()

	_ = os.MkdirAll("/usr/share/phpmyadmin/tmp", 0777)
	_ = os.Chmod("/usr/share/phpmyadmin/tmp", 0777)
	_ = exec.CommandContext(ctx, "chown", "-R", "www-data:www-data", "/usr/share/phpmyadmin/tmp").Run()

	// 4. Ensure required PHP extensions (mbstring, mysql, xml, etc.) are installed and enabled for all active PHP runtimes
	extScript := `
		export DEBIAN_FRONTEND=noninteractive
		need_reload=0
		for sock in /run/php/php*-fpm.sock; do
			if [ -e "$sock" ]; then
				ver=$(basename "$sock" | sed -E 's/php([0-9]+\.[0-9]+)-fpm\.sock/\1/')
				need_pkg=0
				for ext in mbstring mysqli xml curl zip gd; do
					if [ ! -f "/etc/php/$ver/fpm/conf.d/"*"$ext"*.ini ] && ! php$ver -m 2>/dev/null | grep -qi "^$ext$"; then
						need_pkg=1
						break
					fi
				done
				if [ "$need_pkg" -eq 1 ]; then
					apt-get update -y >/dev/null 2>&1 || true
					apt-get install -y php$ver-mbstring php$ver-mysql php$ver-xml php$ver-curl php$ver-zip php$ver-gd || true
					phpenmod -v $ver mbstring mysqli mysql pdo_mysql xml curl zip gd 2>/dev/null || true
					need_reload=1
				fi
			fi
		done

		# Also ensure metapackages if missing
		if ! dpkg -s php-mbstring >/dev/null 2>&1 || ! dpkg -s php-mysql >/dev/null 2>&1; then
			apt-get install -y php-mbstring php-mysql php-xml php-curl php-zip php-gd || true
			need_reload=1
		fi

		phpenmod mbstring mysqli mysql pdo_mysql xml curl zip gd 2>/dev/null || true

		if [ "$need_reload" -eq 1 ]; then
			for svc in $(systemctl list-unit-files --type=service | grep -oE 'php[0-9\.]+-fpm\.service'); do
				systemctl restart "$svc" 2>/dev/null || true
			done
			systemctl reload nginx 2>/dev/null || true
		fi
	`
	_ = exec.CommandContext(ctx, "bash", "-c", extScript).Run()

	// 5. Configure dual-server setup: Server 1 for Standard Cookie login, Server 2 for Seintech SSO
	confFile := "/etc/phpmyadmin/conf.d/01-seintech.inc.php"
	_ = os.MkdirAll("/etc/phpmyadmin/conf.d", 0755)
	customConf := `<?php
/* Auto-generated by Seintech Panel */

if (empty($cfg['blowfish_secret']) || strlen($cfg['blowfish_secret']) < 32) {
    $cfg['blowfish_secret'] = 's31nt3chP4n3lS3cur3Bl0wf1shS3cr3tK3y!';
}

$sock = file_exists('/run/mysqld/mysqld.sock') ? '/run/mysqld/mysqld.sock' : (file_exists('/var/run/mysqld/mysqld.sock') ? '/var/run/mysqld/mysqld.sock' : '');

// Server 1: Standard Login (Cookie) - Default
$cfg['Servers'][1]['auth_type'] = 'cookie';
if (!empty($sock)) {
    $cfg['Servers'][1]['host'] = 'localhost';
    $cfg['Servers'][1]['socket'] = $sock;
    $cfg['Servers'][1]['connect_type'] = 'socket';
} else {
    $cfg['Servers'][1]['host'] = '127.0.0.1';
    $cfg['Servers'][1]['port'] = 3306;
    $cfg['Servers'][1]['connect_type'] = 'tcp';
}
$cfg['Servers'][1]['compress'] = false;
$cfg['Servers'][1]['AllowNoPassword'] = false;
$cfg['Servers'][1]['verbose'] = 'MariaDB / MySQL (Cookie)';

// Server 2: Single Sign-On (SignonScript)
$cfg['Servers'][2]['auth_type'] = 'signon';
$cfg['Servers'][2]['SignonScript'] = '/usr/share/phpmyadmin/signon_auth.php';
$cfg['Servers'][2]['SignonURL'] = '/phpmyadmin/index.php';
if (!empty($sock)) {
    $cfg['Servers'][2]['host'] = 'localhost';
    $cfg['Servers'][2]['socket'] = $sock;
    $cfg['Servers'][2]['connect_type'] = 'socket';
} else {
    $cfg['Servers'][2]['host'] = '127.0.0.1';
    $cfg['Servers'][2]['port'] = 3306;
    $cfg['Servers'][2]['connect_type'] = 'tcp';
}
$cfg['Servers'][2]['compress'] = false;
$cfg['Servers'][2]['AllowNoPassword'] = false;
$cfg['Servers'][2]['verbose'] = 'Seintech SSO';

$cfg['SendErrorReports'] = 'never';
`
	_ = os.WriteFile(confFile, []byte(customConf), 0644)

	// Ensure /usr/share/phpmyadmin/config.inc.php links to configuration
	usrConf := "/usr/share/phpmyadmin/config.inc.php"
	if _, err := os.Stat(usrConf); os.IsNotExist(err) {
		if _, err := os.Stat("/etc/phpmyadmin/config.inc.php"); err == nil {
			_ = os.Symlink("/etc/phpmyadmin/config.inc.php", usrConf)
		} else {
			stub := "<?php\nif (file_exists('/etc/phpmyadmin/conf.d/01-seintech.inc.php')) {\n    include('/etc/phpmyadmin/conf.d/01-seintech.inc.php');\n}\n"
			_ = os.WriteFile(usrConf, []byte(stub), 0644)
		}
	}

	// 6. Install native SignonScript in /usr/share/phpmyadmin/signon_auth.php and /etc/phpmyadmin/signon_auth.php
	signonAuthCode := `<?php
declare(strict_types=1);

/**
 * Seintech Panel SSO Authentication Script for phpMyAdmin
 * Uses official SignonScript interface get_login_credentials
 */
function get_login_credentials(string $user): array
{
    $token = $_COOKIE['seintech_pma_token'] ?? '';
    if (empty($token) && isset($_GET['token'])) {
        $token = $_GET['token'];
    }

    if (!is_string($token) || !preg_match('/^[a-f0-9]{32,64}$/', $token)) {
        return ['', ''];
    }

    $tokenFile = '/var/lib/seintech/pma_tokens/' . $token . '.json';
    if (!file_exists($tokenFile)) {
        return ['', ''];
    }

    $raw = @file_get_contents($tokenFile);
    if (!$raw) {
        return ['', ''];
    }

    $data = @json_decode($raw, true);
    if (!$data || time() > ($data['expires'] ?? 0)) {
        @unlink($tokenFile);
        return ['', ''];
    }

    $u = (string)($data['user'] ?? '');
    $p = (string)($data['pass'] ?? '');

    if ($u === '') {
        return ['', ''];
    }

    return [$u, $p];
}
`
	_ = os.WriteFile("/usr/share/phpmyadmin/signon_auth.php", []byte(signonAuthCode), 0644)
	_ = os.WriteFile("/etc/phpmyadmin/signon_auth.php", []byte(signonAuthCode), 0644)
	_ = exec.CommandContext(ctx, "chown", "www-data:www-data", "/usr/share/phpmyadmin/signon_auth.php").Run()

	// 7. Install SSO bridge script in /usr/share/phpmyadmin/sso.php
	ssoScript := `/usr/share/phpmyadmin/sso.php`
	ssoContent := `<?php
// Seintech Panel SSO Bridge for phpMyAdmin
$token = $_GET['token'] ?? '';
if (!is_string($token) || !preg_match('/^[a-f0-9]{32,64}$/', $token)) {
    die("Invalid token format");
}
$tokenFile = "/var/lib/seintech/pma_tokens/" . $token . ".json";
if (!file_exists($tokenFile)) {
    die("Token not found or already used");
}
$raw = @file_get_contents($tokenFile);
$data = @json_decode($raw, true);
if (!$data || time() > ($data['expires'] ?? 0)) {
    @unlink($tokenFile);
    die("Token expired");
}

$isHttps = (isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] === 'on') || 
           (isset($_SERVER['HTTP_X_FORWARDED_PROTO']) && $_SERVER['HTTP_X_FORWARDED_PROTO'] === 'https');

// Set token cookie so SignonScript get_login_credentials can read it
if (PHP_VERSION_ID >= 70300) {
    setcookie('seintech_pma_token', $token, [
        'expires' => time() + 180,
        'path' => '/phpmyadmin/',
        'httponly' => true,
        'secure' => $isHttps,
        'samesite' => 'Lax'
    ]);
} else {
    setcookie('seintech_pma_token', $token, time() + 180, '/phpmyadmin/', '', $isHttps, true);
}

// Clear any stale phpMyAdmin session cookie so fresh login takes effect
foreach ($_COOKIE as $name => $val) {
    if (stripos($name, 'pma') !== false || stripos($name, 'phpmyadmin') !== false) {
        if ($name !== 'seintech_pma_token') {
            setcookie($name, '', time() - 3600, '/');
            setcookie($name, '', time() - 3600, '/phpmyadmin/');
        }
    }
}

$db = urlencode($data['db'] ?? '');
header("Location: /phpmyadmin/index.php?server=2&db=" . $db . "&token=" . $token);
exit;
`
	_ = os.WriteFile(ssoScript, []byte(ssoContent), 0644)
	_ = exec.CommandContext(ctx, "chown", "www-data:www-data", ssoScript).Run()
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

// EnsureDatabaseTools preseeds debconf and auto-configures both tools if present.
func (sm *ServiceManager) EnsureDatabaseTools(ctx context.Context) {
	sm.ensurePhpMyAdminPrerequisites(ctx)
	sm.PreseedDebconf(ctx)
	sm.AutoConfigurePhpMyAdmin(ctx)
	sm.AutoConfigurePhpPgAdmin(ctx)
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
