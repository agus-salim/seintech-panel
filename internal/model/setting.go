package model

type PanelSettings struct {
	Hostname               string   `json:"hostname"`
	Port                   int      `json:"port"`
	ListenAddr             string   `json:"listen_addr"`
	DefaultWebServer       string   `json:"default_webserver"` // nginx, apache2, openlitespeed, caddy
	DefaultPHPVersion      string   `json:"default_php_version"`
	SessionTimeoutMinutes  int      `json:"session_timeout_minutes"`
	SecurityHeadersEnabled bool     `json:"security_headers_enabled"`
	SSLEnabled             bool     `json:"ssl_enabled"`
	SSLCertPath            string   `json:"ssl_cert_path"`
	SSLKeyPath             string   `json:"ssl_key_path"`
	AutoRenewSSL           bool     `json:"auto_renew_ssl"`
	AllowedIPs             []string `json:"allowed_ips,omitempty"`
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
