package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	repo       *storage.Repository
	registry   *webserver.Registry
	phpMgr     *php.PHPManager
	sslMgr     *ssl.SSLManager
	cfg        *config.Config
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
	Domains []model.Domain
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
	}

	_ = web.RenderTemplate(w, "domains.html", data)
}

func (h *DomainHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	domainName := r.FormValue("domain_name")
	wsType := model.WebServerType(r.FormValue("webserver_type"))
	phpVer := r.FormValue("php_version")
	mode := r.FormValue("mode")
	proxyTarget := r.FormValue("proxy_target")
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

func (h *DomainHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	domains, _ := h.repo.GetDomains()
	for _, d := range domains {
		if d.ID == id {
			ws, err := h.registry.Get(d.WebServerType)
			if err == nil {
				_ = ws.RemoveVHost(r.Context(), &d)
				_ = ws.Reload(r.Context())
			}
			_ = h.repo.DeleteDomain(id)
			break
		}
	}
	http.Redirect(w, r, "/domains?success=deleted", http.StatusSeeOther)
}
