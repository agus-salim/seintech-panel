package model

import "time"

type WebServerType string

const (
	WebServerNginx  WebServerType = "nginx"
	WebServerApache WebServerType = "apache2"
)

type SSLProvider string

const (
	SSLProviderLetsEncrypt SSLProvider = "letsencrypt"
	SSLProviderCustom      SSLProvider = "custom"
	SSLProviderSelfSigned  SSLProvider = "selfsigned"
	SSLProviderNone        SSLProvider = "none"
)

type Domain struct {
	ID             string        `json:"id"`
	UserID         string        `json:"user_id"`
	DomainName     string        `json:"domain_name"`
	Aliases        []string      `json:"aliases,omitempty"`
	DocumentRoot   string        `json:"document_root"`
	WebServerType  WebServerType `json:"webserver_type"`
	PHPVersion     string        `json:"php_version"` // e.g. "8.2", "8.3", "static"
	PHPSocket      string        `json:"php_socket"`
	SSLEnabled     bool          `json:"ssl_enabled"`
	SSLProvider    SSLProvider   `json:"ssl_provider"`
	SSLCertPath    string        `json:"ssl_cert_path,omitempty"`
	SSLKeyPath     string        `json:"ssl_key_path,omitempty"`
	SSLExpiry      *time.Time    `json:"ssl_expiry,omitempty"`
	IsReverseProxy bool          `json:"is_reverse_proxy"`
	ProxyTarget    string        `json:"proxy_target,omitempty"` // e.g. "http://127.0.0.1:3000"
	IsStatic       bool          `json:"is_static"`
	CustomHeaders  []string      `json:"custom_headers,omitempty"`
	SiteUser       string        `json:"site_user,omitempty"`
	SiteGroup      string        `json:"site_group,omitempty"`
	Status         string        `json:"status"` // "active", "disabled"
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
