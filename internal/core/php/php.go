package php

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

		// Check if pool directory or binary exists
		installed := false
		if _, err := os.Stat(poolDir); err == nil {
			installed = true
		} else if _, err := os.Stat(fmt.Sprintf("/usr/bin/php%s", v)); err == nil {
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
