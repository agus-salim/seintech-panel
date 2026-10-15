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
		AptPackages: []string{"php7.4-fpm", "php7.4-cli", "php7.4-mysql", "php7.4-curl", "php7.4-gd", "php7.4-mbstring", "php7.4-xml", "php7.4-zip"},
	},
	{
		Name:        "php8.0-fpm",
		DisplayName: "PHP 8.0 FPM",
		Category:    "php",
		Description: "PHP 8.0 FastCGI Process Manager with JIT compiler support.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.0"},
		AptPackages: []string{"php8.0-fpm", "php8.0-cli", "php8.0-mysql", "php8.0-curl", "php8.0-gd", "php8.0-mbstring", "php8.0-xml", "php8.0-zip"},
	},
	{
		Name:        "php8.1-fpm",
		DisplayName: "PHP 8.1 FPM",
		Category:    "php",
		Description: "PHP 8.1 with enums, readonly properties, and first-class callables.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.1"},
		AptPackages: []string{"php8.1-fpm", "php8.1-cli", "php8.1-mysql", "php8.1-curl", "php8.1-gd", "php8.1-mbstring", "php8.1-xml", "php8.1-zip"},
	},
	{
		Name:        "php8.2-fpm",
		DisplayName: "PHP 8.2 FPM",
		Category:    "php",
		Description: "PHP 8.2 stable release with readonly classes and performance improvements.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.2"},
		AptPackages: []string{"php8.2-fpm", "php8.2-cli", "php8.2-mysql", "php8.2-curl", "php8.2-gd", "php8.2-mbstring", "php8.2-xml", "php8.2-zip"},
	},
	{
		Name:        "php8.3-fpm",
		DisplayName: "PHP 8.3 FPM",
		Category:    "php",
		Description: "PHP 8.3 current standard with typed class constants and json_validate.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.3"},
		AptPackages: []string{"php8.3-fpm", "php8.3-cli", "php8.3-mysql", "php8.3-curl", "php8.3-gd", "php8.3-mbstring", "php8.3-xml", "php8.3-zip"},
	},
	{
		Name:        "php8.4-fpm",
		DisplayName: "PHP 8.4 FPM",
		Category:    "php",
		Description: "PHP 8.4 latest modern runtime with property hooks and asymmetric visibility.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.4"},
		AptPackages: []string{"php8.4-fpm", "php8.4-cli", "php8.4-mysql", "php8.4-curl", "php8.4-gd", "php8.4-mbstring", "php8.4-xml", "php8.4-zip"},
	},
	{
		Name:        "php8.5-fpm",
		DisplayName: "PHP 8.5 FPM",
		Category:    "php",
		Description: "PHP 8.5 developer preview runtime for cutting edge evaluation.",
		CheckPaths:  []string{"/usr/sbin/php-fpm8.5"},
		AptPackages: []string{"php8.5-fpm", "php8.5-cli", "php8.5-mysql", "php8.5-curl", "php8.5-gd", "php8.5-mbstring", "php8.5-xml", "php8.5-zip"},
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
		AptPackages: []string{"phppgadmin"},
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

// GetStatus checks if the service is currently running.
func (sm *ServiceManager) GetStatus(ctx context.Context, serviceName string) (ServiceState, error) {
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
		if sm.IsInstalled(serviceName) {
			return ServiceState{
				Name:   serviceName,
				Status: "running",
				Active: true,
			}, nil
		}
		return ServiceState{
			Name:   serviceName,
			Status: "not_installed",
			Active: false,
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
		return nil
	}
	_, err := sm.ExecuteSystemctl(ctx, "stop", serviceName)
	return err
}

func (sm *ServiceManager) Restart(ctx context.Context, serviceName string) error {
	if serviceName == "phpmyadmin" || serviceName == "phppgadmin" {
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
	if err != nil {
		return fmt.Errorf("package installation failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Post-install port and configuration enforcement
	if serviceName == "apache2" {
		sm.ensureApachePorts(ctx)
	}
	if serviceName == "phpmyadmin" {
		sm.AutoConfigurePhpMyAdmin(ctx)
	}
	if serviceName == "phppgadmin" {
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

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Stop & disable service first (only for daemon services)
	if targetDef.Category != "database_tool" {
		_ = sm.Stop(ctx, targetDef.Name)
		_, _ = sm.ExecuteSystemctl(ctx, "disable", targetDef.Name)
	}

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

	if targetDef.Category == "database_tool" {
		_, _ = sm.ExecuteSystemctl(ctx, "reload", "nginx")
	}

	return nil
}

// PreseedDebconf sets debconf selections non-interactively to prevent any interactive dialogs.
func (sm *ServiceManager) PreseedDebconf(ctx context.Context) {
	preseeds := []string{
		"phpmyadmin phpmyadmin/dbconfig-install boolean false",
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
		"phppgadmin phppgadmin/reconfigure-webserver multiselect none",
		"phppgadmin phppgadmin/dbconfig-install boolean false",
	}

	for _, ps := range preseeds {
		cmd := exec.CommandContext(ctx, "debconf-set-selections")
		cmd.Stdin = strings.NewReader(ps + "\n")
		_ = cmd.Run()
	}
}

// AutoConfigurePhpMyAdmin creates blowfish secret, fixes permissions, and configures phpMyAdmin.
func (sm *ServiceManager) AutoConfigurePhpMyAdmin(ctx context.Context) {
	if !sm.IsInstalled("phpmyadmin") {
		return
	}

	// 1. Ensure blowfish secret file exists and is valid
	secretFile := "/var/lib/phpmyadmin/blowfish_secret.inc.php"
	if fi, err := os.Stat(secretFile); err != nil || fi.Size() < 20 {
		_ = os.MkdirAll("/var/lib/phpmyadmin", 0755)
		bytes := make([]byte, 16)
		_, _ = rand.Read(bytes)
		secret := fmt.Sprintf("%x", bytes) // 32 chars hex
		content := fmt.Sprintf("<?php\n$cfg['blowfish_secret'] = '%s';\n", secret)
		_ = os.WriteFile(secretFile, []byte(content), 0640)
		_ = exec.CommandContext(ctx, "chown", "-R", "www-data:www-data", "/var/lib/phpmyadmin").Run()
	}

	// 2. Ensure tmp directory exists for caching/sessions
	tmpDir := "/var/lib/phpmyadmin/tmp"
	_ = os.MkdirAll(tmpDir, 0777)
	_ = os.Chmod(tmpDir, 0777)

	// 3. Ensure phpmyadmin config allows standard MySQL root & user logins without dbconfig-common errors
	confFile := "/etc/phpmyadmin/conf.d/01-seintech.inc.php"
	_ = os.MkdirAll("/etc/phpmyadmin/conf.d", 0755)
	customConf := `<?php
/* Auto-generated by Seintech Panel */
$cfg['Servers'][1]['auth_type'] = 'cookie';
$cfg['Servers'][1]['host'] = 'localhost';
$cfg['Servers'][1]['compress'] = false;
$cfg['Servers'][1]['AllowNoPassword'] = false;
$cfg['SendErrorReports'] = 'never';
`
	_ = os.WriteFile(confFile, []byte(customConf), 0644)
}

// AutoConfigurePhpPgAdmin adjusts security settings so PostgreSQL logins work seamlessly.
func (sm *ServiceManager) AutoConfigurePhpPgAdmin(ctx context.Context) {
	if !sm.IsInstalled("phppgadmin") {
		return
	}

	confFile := "/etc/phppgadmin/config.inc.php"
	data, err := os.ReadFile(confFile)
	if err != nil {
		return
	}

	str := string(data)
	modified := false

	// Disable extra login security so 'postgres' superuser and local users can log in
	if strings.Contains(str, "$conf['extra_login_security'] = true;") {
		str = strings.ReplaceAll(str, "$conf['extra_login_security'] = true;", "$conf['extra_login_security'] = false;")
		modified = true
	}

	// Allow users to see all databases they have access to
	if strings.Contains(str, "$conf['owned_only'] = true;") {
		str = strings.ReplaceAll(str, "$conf['owned_only'] = true;", "$conf['owned_only'] = false;")
		modified = true
	}

	if modified {
		_ = os.WriteFile(confFile, []byte(str), 0644)
	}
}

// EnsureDatabaseTools preseeds debconf and auto-configures both tools if present.
func (sm *ServiceManager) EnsureDatabaseTools(ctx context.Context) {
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
