package model

import "time"

type PanelHost struct {
	ID          string     `json:"id"`
	Hostname    string     `json:"hostname"`
	Port        int        `json:"port"`
	SSLEnabled  bool       `json:"ssl_enabled"`
	SSLProvider string     `json:"ssl_provider"` // "letsencrypt", "selfsigned", "none"
	SSLExpiry   *time.Time `json:"ssl_expiry,omitempty"`
	IsPrimary   bool       `json:"is_primary"`
}

type GoogleDriveSettings struct {
	Enabled      bool      `json:"enabled"`
	Connected    bool      `json:"connected"`
	Email        string    `json:"email"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RefreshToken string    `json:"refresh_token"`
	AccessToken  string    `json:"access_token,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`
	TargetFolder string    `json:"target_folder"` // default: "SeintechBackups"
	AutoSync     bool      `json:"auto_sync"`     // automatically upload new backups
	LastSyncAt   time.Time `json:"last_sync_at,omitempty"`
}

type PanelSettings struct {
	Hostname               string              `json:"hostname"`
	Port                   int                 `json:"port"`
	ListenAddr             string              `json:"listen_addr"`
	Hosts                  []PanelHost         `json:"hosts,omitempty"`
	DefaultWebServer       string              `json:"default_webserver"` // nginx, apache2
	DefaultPHPVersion      string              `json:"default_php_version"`
	SessionTimeoutMinutes  int                 `json:"session_timeout_minutes"`
	SecurityHeadersEnabled bool                `json:"security_headers_enabled"`
	SSLEnabled             bool                `json:"ssl_enabled"`
	SSLCertPath            string              `json:"ssl_cert_path"`
	SSLKeyPath             string              `json:"ssl_key_path"`
	AutoRenewSSL           bool                `json:"auto_renew_ssl"`
	AllowedIPs             []string            `json:"allowed_ips,omitempty"`
	Language               string              `json:"language,omitempty"` // "en" (default) or "id"
	GoogleDrive            GoogleDriveSettings `json:"google_drive,omitempty"`
}

type PHPConfig struct {
	DomainID           string            `json:"domain_id"`
	Version            string            `json:"version"` // e.g. "8.3"
	UploadMaxFilesize  string            `json:"upload_max_filesize"`
	PostMaxSize        string            `json:"post_max_size"`
	MemoryLimit        string            `json:"memory_limit"`
	MaxExecutionTime   int               `json:"max_execution_time"`
	MaxInputVars       int               `json:"max_input_vars"`
	DisplayErrors      bool              `json:"display_errors"`
	CustomDirectives   map[string]string `json:"custom_directives,omitempty"`
}
