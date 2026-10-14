package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	"lsws":        true,
	"caddy":       true,
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
}

type ServiceDefinition struct {
	Name        string
	DisplayName string
	Category    string
	Description string
	CheckPaths  []string
	AptPackages []string
}

var Catalog = []ServiceDefinition{
	// Web Servers
	{
		Name:        "nginx",
		DisplayName: "Nginx Web Server",
		Category:    "webserver",
		Description: "High-performance HTTP server, reverse proxy, and IMAP/POP3 proxy server.",
		CheckPaths:  []string{"/usr/sbin/nginx", "/etc/nginx"},
		AptPackages: []string{"nginx"},
	},
	{
		Name:        "apache2",
		DisplayName: "Apache 2.4 Server",
		Category:    "webserver",
		Description: "Reliable, open-source HTTP server with modular proxy and rewrite capabilities.",
		CheckPaths:  []string{"/usr/sbin/apache2", "/etc/apache2"},
		AptPackages: []string{"apache2", "apache2-utils"},
	},
	{
		Name:        "caddy",
		DisplayName: "Caddy Server",
		Category:    "webserver",
		Description: "Enterprise-ready open-source web server with automatic HTTPS.",
		CheckPaths:  []string{"/usr/bin/caddy", "/etc/caddy"},
		AptPackages: []string{"caddy"},
	},
	{
		Name:        "lsws",
		DisplayName: "OpenLiteSpeed",
		Category:    "webserver",
		Description: "High-performance, lightweight HTTP server with native LSAPI cache.",
		CheckPaths:  []string{"/usr/local/lsws/bin/lswsctrl", "/usr/local/lsws"},
		AptPackages: []string{"openlitespeed"},
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
		CheckPaths:  []string{"/usr/sbin/php-fpm7.4", "/etc/php/7.4"},
		AptPackages: []string{"php7.4-fpm", "php7.4-cli", "php7.4-mysql", "php7.4-curl", "php7.4-gd", "php7.4-mbstring", "php7.4-xml", "php7.4-zip"},
	},
	{
		Name:        "php8.0-fpm",
		DisplayName: "PHP 8.0 FPM",
		Category:    "php",
		Description: "PHP 8.0 FastCGI Process Manager with JIT compiler support.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.0", "/etc/php/8.0"},
		AptPackages: []string{"php8.0-fpm", "php8.0-cli", "php8.0-mysql", "php8.0-curl", "php8.0-gd", "php8.0-mbstring", "php8.0-xml", "php8.0-zip"},
	},
	{
		Name:        "php8.1-fpm",
		DisplayName: "PHP 8.1 FPM",
		Category:    "php",
		Description: "PHP 8.1 with enums, readonly properties, and first-class callables.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.1", "/etc/php/8.1"},
		AptPackages: []string{"php8.1-fpm", "php8.1-cli", "php8.1-mysql", "php8.1-curl", "php8.1-gd", "php8.1-mbstring", "php8.1-xml", "php8.1-zip"},
	},
	{
		Name:        "php8.2-fpm",
		DisplayName: "PHP 8.2 FPM",
		Category:    "php",
		Description: "PHP 8.2 stable release with readonly classes and performance improvements.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.2", "/etc/php/8.2"},
		AptPackages: []string{"php8.2-fpm", "php8.2-cli", "php8.2-mysql", "php8.2-curl", "php8.2-gd", "php8.2-mbstring", "php8.2-xml", "php8.2-zip"},
	},
	{
		Name:        "php8.3-fpm",
		DisplayName: "PHP 8.3 FPM",
		Category:    "php",
		Description: "PHP 8.3 current standard with typed class constants and json_validate.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.3", "/etc/php/8.3"},
		AptPackages: []string{"php8.3-fpm", "php8.3-cli", "php8.3-mysql", "php8.3-curl", "php8.3-gd", "php8.3-mbstring", "php8.3-xml", "php8.3-zip"},
	},
	{
		Name:        "php8.4-fpm",
		DisplayName: "PHP 8.4 FPM",
		Category:    "php",
		Description: "PHP 8.4 latest modern runtime with property hooks and asymmetric visibility.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.4", "/etc/php/8.4"},
		AptPackages: []string{"php8.4-fpm", "php8.4-cli", "php8.4-mysql", "php8.4-curl", "php8.4-gd", "php8.4-mbstring", "php8.4-xml", "php8.4-zip"},
	},
	{
		Name:        "php8.5-fpm",
		DisplayName: "PHP 8.5 FPM",
		Category:    "php",
		Description: "PHP 8.5 developer preview runtime for cutting edge evaluation.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.5", "/etc/php/8.5"},
		AptPackages: []string{"php8.5-fpm", "php8.5-cli", "php8.5-mysql", "php8.5-curl", "php8.5-gd", "php8.5-mbstring", "php8.5-xml", "php8.5-zip"},
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
		return outStr, fmt.Errorf("systemctl %s %s failed: %w (%s)", action, serviceName, err, outStr)
	}

	return outStr, nil
}

// GetStatus checks if the service is currently running.
func (sm *ServiceManager) GetStatus(ctx context.Context, serviceName string) (ServiceState, error) {
	out, err := sm.ExecuteSystemctl(ctx, "is-active", serviceName)
	state := ServiceState{
		Name:   serviceName,
		Status: out,
		Active: out == "active",
	}
	return state, err
}

func (sm *ServiceManager) Start(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "start", serviceName)
	return err
}

func (sm *ServiceManager) Stop(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "stop", serviceName)
	return err
}

func (sm *ServiceManager) Restart(ctx context.Context, serviceName string) error {
	_, err := sm.ExecuteSystemctl(ctx, "restart", serviceName)
	return err
}

func (sm *ServiceManager) Reload(ctx context.Context, serviceName string) error {
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

	// Special setup for Caddy repository if not present
	if serviceName == "caddy" {
		sm.ensureCaddyRepo(ctx)
	}

	// apt-get update
	updateCmd := exec.CommandContext(ctx, "apt-get", "update", "-y")
	updateCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = updateCmd.Run()

	// apt-get install -y <packages>
	args := append([]string{"install", "-y"}, targetDef.AptPackages...)
	installCmd := exec.CommandContext(ctx, "apt-get", args...)
	installCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("package installation failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Enable and start the installed service
	_, _ = sm.ExecuteSystemctl(ctx, "enable", targetDef.Name)
	_, _ = sm.ExecuteSystemctl(ctx, "start", targetDef.Name)

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

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Stop & disable service first
	_ = sm.Stop(ctx, targetDef.Name)
	_, _ = sm.ExecuteSystemctl(ctx, "disable", targetDef.Name)

	// apt-get purge -y <packages>
	args := append([]string{"purge", "-y"}, targetDef.AptPackages...)
	purgeCmd := exec.CommandContext(ctx, "apt-get", args...)
	purgeCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := purgeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to purge service: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// apt-get autoremove -y
	autoCmd := exec.CommandContext(ctx, "apt-get", "autoremove", "-y")
	autoCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	_ = autoCmd.Run()

	return nil
}

func (sm *ServiceManager) ensureSuryPHPRepo(ctx context.Context) {
	// If the keyring and list already exist and are non-empty, check if already configured
	if stat, err := os.Stat("/etc/apt/sources.list.d/php.list"); err == nil && stat.Size() > 0 {
		if _, err := os.Stat("/usr/share/keyrings/deb.sury.org-php.gpg"); err == nil {
			return
		}
	}

	// Official Sury debian setup for Debian 12 (Bookworm) and Debian 13 (Trixie)
	script := `
		export DEBIAN_FRONTEND=noninteractive
		apt-get update -y
		apt-get install -y ca-certificates curl gnupg lsb-release
		curl -fsSL -o /tmp/debsuryorg-archive-keyring.deb https://packages.sury.org/debsuryorg-archive-keyring.deb
		dpkg -i /tmp/debsuryorg-archive-keyring.deb || true
		rm -f /tmp/debsuryorg-archive-keyring.deb

		CODENAME=""
		if [ -f /etc/os-release ]; then
			. /etc/os-release
			CODENAME="$VERSION_CODENAME"
		fi
		if [ -z "$CODENAME" ] && command -v lsb_release >/dev/null 2>&1; then
			CODENAME=$(lsb_release -sc)
		fi
		if [ -z "$CODENAME" ]; then
			CODENAME="trixie"
		fi

		echo "deb [signed-by=/usr/share/keyrings/deb.sury.org-php.gpg] https://packages.sury.org/php/ $CODENAME main" > /etc/apt/sources.list.d/php.list
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
