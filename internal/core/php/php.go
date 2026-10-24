package php

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
)

var SupportedVersions = []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}

type PHPVersionInfo struct {
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Socket    string `json:"socket"`
	PoolDir   string `json:"pool_dir"`
	IniFile   string `json:"ini_file"`
}

type PHPManager struct {
	sm *system.ServiceManager
}

func NewPHPManager(sm *system.ServiceManager) *PHPManager {
	return &PHPManager{sm: sm}
}

// GetSocketPath returns the standard Unix socket path for a given PHP version.
func (p *PHPManager) GetSocketPath(version string) string {
	return fmt.Sprintf("/run/php/php%s-fpm.sock", version)
}

// GetServiceName returns the systemd unit name for a given PHP version.
func (p *PHPManager) GetServiceName(version string) string {
	return fmt.Sprintf("php%s-fpm", version)
}

// ListVersions scans available PHP versions on the host.
func (p *PHPManager) ListVersions(ctx context.Context) ([]PHPVersionInfo, error) {
	var list []PHPVersionInfo

	for _, v := range SupportedVersions {
		sock := p.GetSocketPath(v)
		poolDir := fmt.Sprintf("/etc/php/%s/fpm/pool.d", v)
		iniFile := fmt.Sprintf("/etc/php/%s/fpm/php.ini", v)

		// Check if FPM daemon binary exists
		installed := false
		if _, err := os.Stat(fmt.Sprintf("/usr/sbin/php-fpm%s", v)); err == nil {
			installed = true
		}

		active := false
		if installed {
			state, err := p.sm.GetStatus(ctx, p.GetServiceName(v))
			if err == nil && state.Active {
				active = true
			}
		}

		list = append(list, PHPVersionInfo{
			Version:   v,
			Installed: installed,
			Active:    active,
			Socket:    sock,
			PoolDir:   poolDir,
			IniFile:   iniFile,
		})
	}

	return list, nil
}

const poolTemplate = `[{{ .User }}_{{ .Domain }}]
user = {{ .User }}
group = {{ .User }}
listen = /run/php/php{{ .Version }}-fpm-{{ .User }}.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

pm = dynamic
pm.max_children = 20
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 3
pm.max_requests = 500

; Per-Domain php.ini overrides
php_admin_value[upload_max_filesize] = {{ .UploadMaxFilesize }}
php_admin_value[post_max_size] = {{ .PostMaxSize }}
php_admin_value[memory_limit] = {{ .MemoryLimit }}
php_admin_value[max_execution_time] = {{ .MaxExecutionTime }}
php_admin_value[open_basedir] = {{ .DocumentRoot }}:/tmp
`

type PoolConfigData struct {
	User              string
	Domain            string
	Version           string
	DocumentRoot      string
	UploadMaxFilesize string
	PostMaxSize       string
	MemoryLimit       string
	MaxExecutionTime  int
}

// ConfigureDomainPool writes an isolated PHP-FPM pool for a user/domain with custom php.ini values.
func (p *PHPManager) ConfigureDomainPool(ctx context.Context, domain *model.Domain, username string, cfg *model.PHPConfig) (string, error) {
	if cfg == nil {
		cfg = &model.PHPConfig{
			Version:           domain.PHPVersion,
			UploadMaxFilesize: "64M",
			PostMaxSize:       "64M",
			MemoryLimit:       "256M",
			MaxExecutionTime:  300,
		}
	}

	data := PoolConfigData{
		User:              username,
		Domain:            domain.DomainName,
		Version:           cfg.Version,
		DocumentRoot:      domain.DocumentRoot,
		UploadMaxFilesize: cfg.UploadMaxFilesize,
		PostMaxSize:       cfg.PostMaxSize,
		MemoryLimit:       cfg.MemoryLimit,
		MaxExecutionTime:  cfg.MaxExecutionTime,
	}

	tmpl, err := template.New("pool").Parse(poolTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	poolDir := fmt.Sprintf("/etc/php/%s/fpm/pool.d", cfg.Version)
	_ = os.MkdirAll(poolDir, 0755)

	poolFile := filepath.Join(poolDir, fmt.Sprintf("%s_%s.conf", username, domain.DomainName))
	if err := os.WriteFile(poolFile, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed writing pool config: %w", err)
	}

	// Reload PHP-FPM
	serviceName := p.GetServiceName(cfg.Version)
	_ = p.sm.Reload(ctx, serviceName)

	customSock := fmt.Sprintf("/run/php/php%s-fpm-%s.sock", cfg.Version, username)
	return customSock, nil
}

func (p *PHPManager) Reload(ctx context.Context, version string) error {
	return p.sm.Reload(ctx, p.GetServiceName(version))
}

func (p *PHPManager) Restart(ctx context.Context, version string) error {
	return p.sm.Restart(ctx, p.GetServiceName(version))
}

type ExtensionDefinition struct {
	Name        string `json:"name"`
	PackageName string `json:"package_name,omitempty"`
	Description string `json:"description"`
}

type PHPExtensionStatus struct {
	Name        string `json:"name"`
	PackageName string `json:"package_name"`
	Installed   bool   `json:"installed"`
	Description string `json:"description"`
}

var StandardExtensions = []ExtensionDefinition{
	{Name: "bcmath", Description: "Arbitrary precision mathematics"},
	{Name: "bz2", Description: "Bzip2 compression stream wrapper"},
	{Name: "curl", Description: "cURL HTTP client & data transfer library"},
	{Name: "exif", Description: "Read EXIF image header metadata"},
	{Name: "gd", Description: "GD image processing & manipulation library"},
	{Name: "gmp", Description: "GNU Multiple Precision arithmetic"},
	{Name: "imagick", Description: "ImageMagick graphics manipulation library"},
	{Name: "imap", Description: "IMAP, POP3, and NNTP email protocol support"},
	{Name: "intl", Description: "Internationalization (ICU) functions"},
	{Name: "ioncube", PackageName: "ioncube-loader", Description: "ionCube Loader for running encoded and secured PHP files"},
	{Name: "ldap", Description: "Lightweight Directory Access Protocol"},
	{Name: "mbstring", Description: "Multibyte string handling"},
	{Name: "memcached", Description: "Memcached distributed caching client"},
	{Name: "mongodb", Description: "MongoDB NoSQL database driver"},
	{Name: "mysql", Description: "MySQL & MariaDB PDO/MySQLi database drivers"},
	{Name: "opcache", Description: "Zend OPcache opcode caching accelerator"},
	{Name: "pgsql", Description: "PostgreSQL & PDO_PGSQL database drivers"},
	{Name: "readline", Description: "GNU Readline command-line input"},
	{Name: "redis", Description: "Redis in-memory caching & session store"},
	{Name: "soap", Description: "SOAP XML protocol client & server"},
	{Name: "sqlite3", Description: "SQLite3 & PDO_SQLITE database drivers"},
	{Name: "tidy", Description: "HTML clean-up, repair & traversal"},
	{Name: "xml", Description: "DOM, SimpleXML, XMLReader, XMLWriter"},
	{Name: "xmlrpc", Description: "XML-RPC protocol functions"},
	{Name: "yaml", Description: "YAML data serialization & parsing"},
	{Name: "zip", Description: "Zip archive compression & reading"},
}

// GetExtensions returns the list of installed and available extensions for a given PHP version.
func (p *PHPManager) GetExtensions(ctx context.Context, version string) ([]PHPExtensionStatus, error) {
	// First gather loaded modules from CLI if available
	loadedModules := make(map[string]bool)
	phpBin := fmt.Sprintf("/usr/bin/php%s", version)
	if _, err := os.Stat(phpBin); err == nil {
		out, err := exec.CommandContext(ctx, phpBin, "-m").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				m := strings.ToLower(strings.TrimSpace(line))
				if m != "" && !strings.HasPrefix(m, "[") {
					loadedModules[m] = true
					if strings.Contains(m, "ioncube") {
						loadedModules["ioncube"] = true
					}
				}
			}
		}
	}

	modsAvailableDir := fmt.Sprintf("/etc/php/%s/mods-available", version)
	fpmConfDir := fmt.Sprintf("/etc/php/%s/fpm/conf.d", version)

	var result []PHPExtensionStatus

	for _, ext := range StandardExtensions {
		pkgName := ext.PackageName
		if pkgName == "" {
			pkgName = fmt.Sprintf("php%s-%s", version, ext.Name)
		}

		installed := false

		// 1. Check loaded modules map
		if loadedModules[ext.Name] {
			installed = true
		}

		// Aliases in php -m
		switch ext.Name {
		case "ioncube":
			if loadedModules["ioncube"] || loadedModules["ioncube loader"] || loadedModules["the ioncube php loader"] {
				installed = true
			}
		case "mysql":
			if loadedModules["mysqli"] || loadedModules["pdo_mysql"] || loadedModules["mysqlnd"] {
				installed = true
			}
		case "sqlite3":
			if loadedModules["sqlite3"] || loadedModules["pdo_sqlite"] {
				installed = true
			}
		case "pgsql":
			if loadedModules["pgsql"] || loadedModules["pdo_pgsql"] {
				installed = true
			}
		case "xml":
			if loadedModules["xml"] || loadedModules["dom"] || loadedModules["simplexml"] {
				installed = true
			}
		case "opcache":
			if loadedModules["zend opcache"] || loadedModules["opcache"] {
				installed = true
			}
		}

		// 2. Check /etc/php/<version>/mods-available/<ext>.ini
		if !installed {
			iniPath := filepath.Join(modsAvailableDir, fmt.Sprintf("%s.ini", ext.Name))
			if _, err := os.Stat(iniPath); err == nil {
				installed = true
			} else if ext.Name == "mysql" {
				if _, err := os.Stat(filepath.Join(modsAvailableDir, "mysqli.ini")); err == nil {
					installed = true
				}
			} else if ext.Name == "xml" {
				if _, err := os.Stat(filepath.Join(modsAvailableDir, "xml.ini")); err == nil {
					installed = true
				}
			}
		}

		// 3. Check /etc/php/<version>/fpm/conf.d/
		if !installed {
			matches, _ := filepath.Glob(filepath.Join(fpmConfDir, fmt.Sprintf("*%s*.ini", ext.Name)))
			if len(matches) > 0 {
				installed = true
			}
		}

		result = append(result, PHPExtensionStatus{
			Name:        ext.Name,
			PackageName: pkgName,
			Installed:   installed,
			Description: ext.Description,
		})
	}

	return result, nil
}

// InstallExtension installs a PHP extension package via apt-get or custom installer and reloads PHP-FPM.
func (p *PHPManager) InstallExtension(ctx context.Context, version, extName string) error {
	extName = strings.ToLower(strings.TrimSpace(extName))
	if extName == "ioncube" || extName == "ioncube-loader" {
		return p.InstallIonCube(ctx, version)
	}

	pkg := fmt.Sprintf("php%s-%s", version, extName)

	if _, err := exec.LookPath("apt-get"); err != nil {
		// Mock on systems without apt-get (e.g. during local tests)
		return nil
	}

	cmd := exec.CommandContext(ctx, "apt-get",
		"-o", "DPkg::Lock::Timeout=120",
		"-o", "APT::Get::Lock-Timeout=120",
		"install", "-y", pkg)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menginstall paket %s: %s (%w)", pkg, string(output), err)
	}

	// Restart PHP-FPM service
	_ = p.sm.Restart(ctx, p.GetServiceName(version))
	return nil
}

// DeleteExtension removes a PHP extension package via apt-get or custom uninstaller and reloads PHP-FPM.
func (p *PHPManager) DeleteExtension(ctx context.Context, version, extName string) error {
	extName = strings.ToLower(strings.TrimSpace(extName))
	if extName == "ioncube" || extName == "ioncube-loader" {
		return p.DeleteIonCube(ctx, version)
	}

	pkg := fmt.Sprintf("php%s-%s", version, extName)

	if _, err := exec.LookPath("apt-get"); err != nil {
		// Mock on systems without apt-get (e.g. during local tests)
		return nil
	}

	cmd := exec.CommandContext(ctx, "apt-get",
		"-o", "DPkg::Lock::Timeout=120",
		"-o", "APT::Get::Lock-Timeout=120",
		"remove", "-y", pkg)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus paket %s: %s (%w)", pkg, string(output), err)
	}

	// Restart PHP-FPM service
	_ = p.sm.Restart(ctx, p.GetServiceName(version))
	return nil
}

// InstallIonCube installs ionCube Loader for a given PHP version.
func (p *PHPManager) InstallIonCube(ctx context.Context, version string) error {
	loaderFileName := fmt.Sprintf("ioncube_loader_lin_%s.so", version)
	destDir := "/usr/local/ioncube"
	destFile := filepath.Join(destDir, loaderFileName)

	// If destination file doesn't exist, search candidate source locations
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		candidates := []string{
			filepath.Join("ioncube", loaderFileName),
			filepath.Join("/root/seintech-panel/ioncube", loaderFileName),
			filepath.Join("/var/lib/seintech/ioncube", loaderFileName),
		}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates,
				filepath.Join(filepath.Dir(exe), "ioncube", loaderFileName),
				filepath.Join(filepath.Dir(exe), "..", "ioncube", loaderFileName),
			)
		}

		found := false
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				_ = os.MkdirAll(destDir, 0755)
				if data, err := os.ReadFile(cand); err == nil {
					if err := os.WriteFile(destFile, data, 0755); err == nil {
						found = true
						break
					}
				}
			}
		}

		if !found {
			if _, err := os.Stat(destFile); os.IsNotExist(err) {
				return fmt.Errorf("file %s tidak ditemukan. ionCube Loader hanya mendukung PHP 7.4, 8.1, 8.2, 8.3, 8.4, 8.5", loaderFileName)
			}
		}
	}

	// Prepare ini content with priority 00 so it loads before Zend OPcache
	iniContent := fmt.Sprintf("; configuration for php ioncube loader\n; priority=00\nzend_extension = %s\n", destFile)

	modsDir := fmt.Sprintf("/etc/php/%s/mods-available", version)
	fpmConfDir := fmt.Sprintf("/etc/php/%s/fpm/conf.d", version)
	cliConfDir := fmt.Sprintf("/etc/php/%s/cli/conf.d", version)

	_ = os.MkdirAll(modsDir, 0755)
	_ = os.MkdirAll(fpmConfDir, 0755)
	_ = os.MkdirAll(cliConfDir, 0755)

	_ = os.WriteFile(filepath.Join(modsDir, "ioncube.ini"), []byte(iniContent), 0644)
	_ = os.WriteFile(filepath.Join(fpmConfDir, "00-ioncube.ini"), []byte(iniContent), 0644)
	_ = os.WriteFile(filepath.Join(cliConfDir, "00-ioncube.ini"), []byte(iniContent), 0644)

	if _, err := exec.LookPath("phpenmod"); err == nil {
		_ = exec.CommandContext(ctx, "phpenmod", "-v", version, "ioncube").Run()
	}

	// Restart PHP-FPM service
	_ = p.sm.Restart(ctx, p.GetServiceName(version))
	return nil
}

// DeleteIonCube removes ionCube Loader configuration for a given PHP version.
func (p *PHPManager) DeleteIonCube(ctx context.Context, version string) error {
	fpmConfDir := fmt.Sprintf("/etc/php/%s/fpm/conf.d", version)
	cliConfDir := fmt.Sprintf("/etc/php/%s/cli/conf.d", version)
	modsDir := fmt.Sprintf("/etc/php/%s/mods-available", version)

	_ = os.Remove(filepath.Join(fpmConfDir, "00-ioncube.ini"))
	_ = os.Remove(filepath.Join(fpmConfDir, "ioncube.ini"))
	_ = os.Remove(filepath.Join(cliConfDir, "00-ioncube.ini"))
	_ = os.Remove(filepath.Join(cliConfDir, "ioncube.ini"))
	_ = os.Remove(filepath.Join(modsDir, "ioncube.ini"))

	// Glob remove any *ioncube*.ini
	fpmMatches, _ := filepath.Glob(filepath.Join(fpmConfDir, "*ioncube*.ini"))
	for _, m := range fpmMatches {
		_ = os.Remove(m)
	}
	cliMatches, _ := filepath.Glob(filepath.Join(cliConfDir, "*ioncube*.ini"))
	for _, m := range cliMatches {
		_ = os.Remove(m)
	}

	if _, err := exec.LookPath("phpdismod"); err == nil {
		_ = exec.CommandContext(ctx, "phpdismod", "-v", version, "ioncube").Run()
	}

	// Restart PHP-FPM service
	_ = p.sm.Restart(ctx, p.GetServiceName(version))
	return nil
}

// CommonDirectives to track in Quick Settings
var CommonDirectives = []string{
	"upload_max_filesize",
	"post_max_size",
	"memory_limit",
	"max_execution_time",
	"max_input_time",
	"max_input_vars",
	"display_errors",
	"date.timezone",
	"short_open_tag",
	"default_socket_timeout",
	"allow_url_fopen",
	"opcache.enable",
}

func (p *PHPManager) GetIniPath(version string) string {
	fpmPath := fmt.Sprintf("/etc/php/%s/fpm/php.ini", version)
	if _, err := os.Stat(fpmPath); err == nil {
		return fpmPath
	}
	cliPath := fmt.Sprintf("/etc/php/%s/cli/php.ini", version)
	if _, err := os.Stat(cliPath); err == nil {
		return cliPath
	}
	return fpmPath
}

func (p *PHPManager) GetIni(ctx context.Context, version string) (map[string]any, error) {
	iniPath := p.GetIniPath(version)
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return nil, fmt.Errorf("file konfigurasi %s tidak ditemukan: %w", iniPath, err)
	}

	content := string(data)
	directives := make(map[string]string)

	// Initialize default keys
	for _, d := range CommonDirectives {
		directives[d] = ""
	}

	// Regex to match active directives: ^\s*([a-zA-Z0-9_\.]+)\s*=\s*(.*?)\s*(;.*)?$
	re := regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_\.]+)\s*=\s*([^;\r\n]*?)(\s*;.*)?$`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			key := strings.TrimSpace(m[1])
			val := strings.TrimSpace(m[2])
			val = strings.Trim(val, `"'`)
			directives[key] = val
		}
	}

	return map[string]any{
		"version":    version,
		"file":       iniPath,
		"content":    content,
		"directives": directives,
	}, nil
}

func (p *PHPManager) SaveIniRaw(ctx context.Context, version, newContent string) error {
	iniPath := p.GetIniPath(version)
	if strings.TrimSpace(newContent) == "" {
		return fmt.Errorf("konten konfigurasi php.ini tidak boleh kosong")
	}

	// Backup old ini
	if oldData, err := os.ReadFile(iniPath); err == nil {
		_ = os.WriteFile(iniPath+".bak", oldData, 0644)
	}

	if err := os.WriteFile(iniPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("gagal menulis file %s: %w", iniPath, err)
	}

	// Restart FPM service
	_ = p.sm.Restart(ctx, p.GetServiceName(version))
	return nil
}

func (p *PHPManager) SaveIniDirectives(ctx context.Context, version string, updates map[string]string) error {
	iniPath := p.GetIniPath(version)
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return fmt.Errorf("file konfigurasi %s tidak ditemukan: %w", iniPath, err)
	}

	// Backup
	_ = os.WriteFile(iniPath+".bak", data, 0644)

	content := string(data)
	for key, val := range updates {
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}

		// Pattern to find active or commented directive: ^[;\s]*KEY\s*=.*$
		re := regexp.MustCompile(`(?mi)^[;\s]*` + regexp.QuoteMeta(key) + `\s*=.*$`)
		replacement := fmt.Sprintf("%s = %s", key, val)

		if re.MatchString(content) {
			content = re.ReplaceAllString(content, replacement)
		} else {
			// Append under [PHP] or at end of file
			if idx := strings.Index(content, "[PHP]"); idx != -1 {
				content = content[:idx+5] + "\n" + replacement + content[idx+5:]
			} else {
				content += "\n" + replacement + "\n"
			}
		}
	}

	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("gagal menyimpan %s: %w", iniPath, err)
	}

	// Restart FPM service
	_ = p.sm.Restart(ctx, p.GetServiceName(version))
	return nil
}

