package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/php"
	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type DomainHandler struct {
	repo     *storage.Repository
	registry *webserver.Registry
	phpMgr   *php.PHPManager
	sslMgr   *ssl.SSLManager
	cfg      *config.Config
}

func NewDomainHandler(repo *storage.Repository, registry *webserver.Registry, phpMgr *php.PHPManager, sslMgr *ssl.SSLManager, cfg *config.Config) *DomainHandler {
	return &DomainHandler{
		repo:     repo,
		registry: registry,
		phpMgr:   phpMgr,
		sslMgr:   sslMgr,
		cfg:      cfg,
	}
}

type DomainsPageData struct {
	BasePageData
	Domains     []model.Domain
	PHPVersions []string
	WebServers  []model.WebServerType
}

func (h *DomainHandler) List(w http.ResponseWriter, r *http.Request) {
	domains, _ := h.repo.GetDomains()
	settings, _ := h.repo.GetSettings()

	// Authentically verify each domain's SSL certificate on disk
	for i := range domains {
		_ = h.sslMgr.SyncDomainSSLStatus(&domains[i])
	}

	data := DomainsPageData{
		BasePageData: NewBaseData(r, "Virtual Hosts", "Virtual Hosts & Web Routing", "domains", settings.Hostname),
		Domains:      domains,
		PHPVersions:  php.SupportedVersions,
		WebServers: []model.WebServerType{
			model.WebServerNginx,
			model.WebServerApache,
			model.WebServerCaddy,
			model.WebServerOLS,
		},
	}

	_ = web.RenderTemplate(w, "domains.html", data)
}

func (h *DomainHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	domainName := strings.TrimSpace(r.FormValue("domain_name"))
	wsType := model.WebServerType(strings.TrimSpace(r.FormValue("webserver_type")))
	phpVer := strings.TrimSpace(r.FormValue("php_version"))
	mode := strings.TrimSpace(r.FormValue("mode"))
	proxyTarget := strings.TrimSpace(r.FormValue("proxy_target"))
	sslEnable := r.FormValue("ssl_enable") == "true"

	if domainName == "" {
		http.Redirect(w, r, "/domains?msg=Nama+domain+wajib+diisi&type=error", http.StatusSeeOther)
		return
	}

	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	domainID := hex.EncodeToString(idBytes)

	sess := GetSession(r)
	docRoot := filepath.Join(h.cfg.WebRoot, domainName, "public_html")
	_ = os.MkdirAll(docRoot, 0755)

	// Create default index.php or index.html if empty
	indexFile := filepath.Join(docRoot, "index.php")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		defaultContent := "<?php echo '<h1>Welcome to " + domainName + "</h1><p>Powered by Seintech Hosting Panel</p>'; phpinfo();"
		_ = os.WriteFile(indexFile, []byte(defaultContent), 0644)
	}

	domain := model.Domain{
		ID:             domainID,
		UserID:         sess.UserID,
		DomainName:     domainName,
		DocumentRoot:   docRoot,
		WebServerType:  wsType,
		PHPVersion:     phpVer,
		PHPSocket:      h.phpMgr.GetSocketPath(phpVer),
		SSLEnabled:     false,
		SSLProvider:    model.SSLProviderNone,
		IsReverseProxy: mode == "proxy",
		ProxyTarget:    proxyTarget,
		IsStatic:       mode == "static",
		Status:         "active",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Generate VHost on the chosen WebServer engine
	ws, err := h.registry.Get(wsType)
	if err == nil {
		_ = ws.GenerateVHost(r.Context(), &domain)
		_ = ws.Reload(r.Context())
	}

	// If SSL requested, issue certificate
	if sslEnable {
		err := h.sslMgr.ProvisionLetsEncrypt(r.Context(), &domain)
		if err != nil {
			_ = h.repo.UpsertDomain(domain)
			http.Redirect(w, r, "/domains?msg=Virtual+host+dibuat,+namun+Let's+Encrypt+belum+aktif:+"+url.QueryEscape(err.Error())+"&type=warning", http.StatusSeeOther)
			return
		}
	} else {
		_ = h.repo.UpsertDomain(domain)
	}

	http.Redirect(w, r, "/domains?msg=Virtual+host+berhasil+dibuat&type=success", http.StatusSeeOther)
}

// UpdateEngine handles quick dropdown change of web server engine
func (h *DomainHandler) UpdateEngine(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondJSON(w, false, "Invalid form data")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	newEngine := model.WebServerType(strings.TrimSpace(r.FormValue("webserver_type")))

	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
		if err != nil {
			h.respondJSON(w, false, "Domain tidak ditemukan")
			return
		}
	}

	if domain.WebServerType == newEngine {
		h.respondJSON(w, true, "Web engine tidak berubah")
		return
	}

	// Remove VHost from old webserver
	oldWs, err := h.registry.Get(domain.WebServerType)
	if err == nil {
		_ = oldWs.RemoveVHost(r.Context(), domain)
		_ = oldWs.Reload(r.Context())
	}

	// Update domain model
	oldType := domain.WebServerType
	domain.WebServerType = newEngine
	domain.UpdatedAt = time.Now()

	// Generate VHost on new webserver
	newWs, err := h.registry.Get(newEngine)
	if err != nil {
		h.respondJSON(w, false, fmt.Sprintf("Webserver engine '%s' tidak didukung", newEngine))
		return
	}

	if err := newWs.GenerateVHost(r.Context(), domain); err != nil {
		h.respondJSON(w, false, "Gagal membuat vhost pada "+string(newEngine)+": "+err.Error())
		return
	}
	_ = newWs.Reload(r.Context())

	_ = h.repo.UpsertDomain(*domain)

	msg := fmt.Sprintf("Web engine untuk %s berhasil diubah dari %s ke %s!", domain.DomainName, strings.ToUpper(string(oldType)), strings.ToUpper(string(newEngine)))
	h.respondJSON(w, true, msg)
}

// UpdatePHP handles quick dropdown change of PHP version
func (h *DomainHandler) UpdatePHP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondJSON(w, false, "Invalid form data")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	phpVer := strings.TrimSpace(r.FormValue("php_version"))

	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
		if err != nil {
			h.respondJSON(w, false, "Domain tidak ditemukan")
			return
		}
	}

	domain.PHPVersion = phpVer
	domain.PHPSocket = h.phpMgr.GetSocketPath(phpVer)
	domain.UpdatedAt = time.Now()

	// Regenerate VHost on current webserver
	ws, err := h.registry.Get(domain.WebServerType)
	if err == nil {
		_ = ws.GenerateVHost(r.Context(), domain)
		_ = ws.Reload(r.Context())
	}

	_ = h.repo.UpsertDomain(*domain)

	msg := fmt.Sprintf("Versi PHP untuk %s berhasil diubah ke PHP %s!", domain.DomainName, phpVer)
	h.respondJSON(w, true, msg)
}

// Update handles full edit modal submission
func (h *DomainHandler) Update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/domains?msg=Invalid+form+data&type=error", http.StatusSeeOther)
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
		if err != nil {
			http.Redirect(w, r, "/domains?msg=Domain+tidak+ditemukan&type=error", http.StatusSeeOther)
			return
		}
	}

	newWsType := model.WebServerType(strings.TrimSpace(r.FormValue("webserver_type")))
	phpVer := strings.TrimSpace(r.FormValue("php_version"))
	mode := strings.TrimSpace(r.FormValue("mode"))
	proxyTarget := strings.TrimSpace(r.FormValue("proxy_target"))
	docRoot := strings.TrimSpace(r.FormValue("document_root"))
	aliasesRaw := strings.TrimSpace(r.FormValue("aliases"))

	var aliases []string
	for _, a := range strings.Split(aliasesRaw, ",") {
		cleanA := strings.TrimSpace(a)
		if cleanA != "" && cleanA != domain.DomainName {
			aliases = append(aliases, cleanA)
		}
	}

	if docRoot != "" {
		_ = os.MkdirAll(docRoot, 0755)
		domain.DocumentRoot = docRoot
	}

	oldWsType := domain.WebServerType
	domain.Aliases = aliases
	domain.IsReverseProxy = (mode == "proxy")
	domain.ProxyTarget = proxyTarget
	domain.IsStatic = (mode == "static")
	if phpVer != "" {
		domain.PHPVersion = phpVer
		domain.PHPSocket = h.phpMgr.GetSocketPath(phpVer)
	}
	domain.WebServerType = newWsType
	domain.UpdatedAt = time.Now()

	// If engine changed, remove from old
	if oldWsType != newWsType {
		if oldWs, err := h.registry.Get(oldWsType); err == nil {
			_ = oldWs.RemoveVHost(r.Context(), domain)
			_ = oldWs.Reload(r.Context())
		}
	}

	// Generate on active engine
	if ws, err := h.registry.Get(newWsType); err == nil {
		_ = ws.GenerateVHost(r.Context(), domain)
		_ = ws.Reload(r.Context())
	}

	_ = h.repo.UpsertDomain(*domain)

	http.Redirect(w, r, "/domains?msg=Konfigurasi+virtual+host+berhasil+diperbarui&type=success", http.StatusSeeOther)
}

// GetLogs returns the latest access or error log lines for a domain
func (h *DomainHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	logType := strings.TrimSpace(r.URL.Query().Get("type")) // "access" or "error"
	if logType == "" {
		logType = "access"
	}

	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
		if err != nil {
			http.Error(w, "Domain tidak ditemukan", http.StatusNotFound)
			return
		}
	}

	var logPath string
	switch domain.WebServerType {
	case model.WebServerNginx:
		if logType == "error" {
			logPath = filepath.Join("/var/log/nginx", domain.DomainName+".error.log")
		} else {
			logPath = filepath.Join("/var/log/nginx", domain.DomainName+".access.log")
		}
	case model.WebServerApache:
		if logType == "error" {
			logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_ssl_error.log")
			if _, err := os.Stat(logPath); err != nil {
				logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_error.log")
			}
		} else {
			logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_ssl_access.log")
			if _, err := os.Stat(logPath); err != nil {
				logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_access.log")
			}
		}
	case model.WebServerCaddy:
		logPath = filepath.Join("/var/log/caddy", domain.DomainName+".log")
	case model.WebServerOLS:
		if logType == "error" {
			logPath = filepath.Join("/usr/local/lsws/logs", domain.DomainName+".error.log")
		} else {
			logPath = filepath.Join("/usr/local/lsws/logs", domain.DomainName+".access.log")
		}
	}

	content := readLastLines(logPath, 80)
	if strings.TrimSpace(content) == "" {
		content = fmt.Sprintf("[%s Log kosong atau belum ada data tercatat di %s]", strings.ToUpper(logType), logPath)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"domain": domain.DomainName,
		"engine": domain.WebServerType,
		"type":   logType,
		"path":   logPath,
		"logs":   content,
	})
}

func readLastLines(path string, maxLines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// Delete removes virtual host requiring text "konfirmasi"
func (h *DomainHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/domains?msg=Invalid+request&type=error", http.StatusSeeOther)
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	confirmText := strings.ToLower(strings.TrimSpace(r.FormValue("confirm")))

	if confirmText != "konfirmasi" {
		http.Redirect(w, r, "/domains?msg=Penghapusan+dibatalkan.+Harap+ketik+'konfirmasi'+untuk+menghapus.&type=error", http.StatusSeeOther)
		return
	}

	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
	}

	if domain != nil {
		ws, err := h.registry.Get(domain.WebServerType)
		if err == nil {
			_ = ws.RemoveVHost(r.Context(), domain)
			_ = ws.Reload(r.Context())
		}
		_ = h.repo.DeleteDomain(domain.ID)
		http.Redirect(w, r, fmt.Sprintf("/domains?msg=Virtual+host+%s+berhasil+dihapus&type=success", domain.DomainName), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/domains?msg=Domain+tidak+ditemukan&type=error", http.StatusSeeOther)
}

func (h *DomainHandler) respondJSON(w http.ResponseWriter, success bool, message string) {
	w.Header().Set("Content-Type", "application/json")
	if !success {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   message,
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": message,
	})
}
