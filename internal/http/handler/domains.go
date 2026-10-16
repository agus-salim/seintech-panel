package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/php"
	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/system"
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
	sm       *system.ServiceManager
}

func NewDomainHandler(repo *storage.Repository, registry *webserver.Registry, phpMgr *php.PHPManager, sslMgr *ssl.SSLManager, cfg *config.Config, sm *system.ServiceManager) *DomainHandler {
	return &DomainHandler{
		repo:     repo,
		registry: registry,
		phpMgr:   phpMgr,
		sslMgr:   sslMgr,
		cfg:      cfg,
		sm:       sm,
	}
}

type DomainViewItem struct {
	model.Domain
	AvailablePHPVersions []string
	IsPHPActive          bool
}

type DomainsPageData struct {
	BasePageData
	Domains         []DomainViewItem
	PHPVersions     []string
	WebServers      []model.WebServerType
	CanCreateDomain bool
	UserAccounts    []model.User
}

func (h *DomainHandler) List(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	accessibleIDs := GetAccessibleUserIDs(sess, h.repo)

	allDomains, _ := h.repo.GetDomains()
	var domains []model.Domain
	for _, d := range allDomains {
		if accessibleIDs == nil || accessibleIDs[d.UserID] {
			domains = append(domains, d)
		}
	}

	settings, _ := h.repo.GetSettings()

	// Authentically verify each domain's SSL certificate on disk
	for i := range domains {
		_ = h.sslMgr.SyncDomainSSLStatus(&domains[i])
	}

	// Fetch PHP versions whose service is actively running on the host
	phpList, err := h.phpMgr.ListVersions(r.Context())
	var activePHPVersions []string
	activeMap := make(map[string]bool)
	if err == nil {
		for _, pv := range phpList {
			if pv.Active {
				activePHPVersions = append(activePHPVersions, pv.Version)
				activeMap[pv.Version] = true
			}
		}
	}

	// Sort descending so newer versions appear first: 8.5, 8.4, 8.3...
	sort.Slice(activePHPVersions, func(i, j int) bool {
		return activePHPVersions[i] > activePHPVersions[j]
	})

	// If no PHP service is active, fallback so select isn't empty
	fallbackVersions := activePHPVersions
	if len(fallbackVersions) == 0 {
		fallbackVersions = []string{"8.4", "8.3", "8.2", "8.1", "8.0", "7.4"}
	}

	var domainViews []DomainViewItem
	for _, d := range domains {
		isPHPActive := activeMap[d.PHPVersion]

		// Dropdown options for this domain: list active PHP versions
		opts := append([]string{}, activePHPVersions...)

		// If this domain's assigned PHP version is currently inactive, also include it so the current value is visible
		if d.PHPVersion != "" && !activeMap[d.PHPVersion] {
			opts = append(opts, d.PHPVersion)
			sort.Slice(opts, func(i, j int) bool {
				return opts[i] > opts[j]
			})
		}

		if len(opts) == 0 {
			opts = fallbackVersions
		}

		domainViews = append(domainViews, DomainViewItem{
			Domain:               d,
			AvailablePHPVersions: opts,
			IsPHPActive:          isPHPActive,
		})
	}

	// User Accounts for assigning domain ownership (admin / reseller)
	var manageableUsers []model.User
	if sess.Role != model.RoleUser {
		allUsers, _ := h.repo.GetUsers()
		for _, u := range allUsers {
			if accessibleIDs == nil || accessibleIDs[u.ID] {
				manageableUsers = append(manageableUsers, u)
			}
		}
	}

	canCreate := true
	if sess.Role == model.RoleUser && len(domainViews) >= 1 {
		canCreate = false
	} else if sess.Role == model.RoleReseller {
		rq, _ := h.repo.GetQuota(sess.UserID)
		if rq != nil && rq.MaxDomains > 0 && len(domainViews) >= rq.MaxDomains {
			canCreate = false
		}
	}

	data := DomainsPageData{
		BasePageData:    NewBaseData(r, "Virtual Hosts", "Virtual Hosts & Web Routing", "domains", settings.Hostname),
		Domains:         domainViews,
		PHPVersions:     fallbackVersions,
		CanCreateDomain: canCreate,
		UserAccounts:    manageableUsers,
		WebServers: []model.WebServerType{
			model.WebServerNginx,
			model.WebServerApache,
		},
	}

	_ = web.RenderTemplate(w, "domains.html", data)
}

func (h *DomainHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	sess := GetSession(r)
	accessibleIDs := GetAccessibleUserIDs(sess, h.repo)

	// Check domain quota limits:
	// 1. End-User can only have exactly 1 domain
	if sess.Role == model.RoleUser {
		userDomains, _ := h.repo.GetDomainsByUser(sess.UserID)
		if len(userDomains) >= 1 {
			http.Redirect(w, r, "/domains?msg="+url.QueryEscape("End-User hanya diperbolehkan memiliki 1 situs virtual host.")+"&type=error", http.StatusSeeOther)
			return
		}
	} else if sess.Role == model.RoleReseller {
		// Reseller domain quota check
		rq, _ := h.repo.GetQuota(sess.UserID)
		if rq != nil && rq.MaxDomains > 0 {
			allDomains, _ := h.repo.GetDomains()
			count := 0
			for _, d := range allDomains {
				if accessibleIDs[d.UserID] {
					count++
				}
			}
			if count >= rq.MaxDomains {
				http.Redirect(w, r, "/domains?msg="+url.QueryEscape(fmt.Sprintf("Batas kuota domain reseller (%d domain) telah tercapai.", rq.MaxDomains))+"&type=error", http.StatusSeeOther)
				return
			}
		}
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

	// Determine owner user ID
	targetUserID := sess.UserID
	if sess.Role != model.RoleUser && r.FormValue("owner_id") != "" {
		reqOwner := strings.TrimSpace(r.FormValue("owner_id"))
		if IsUserAccessible(sess, h.repo, reqOwner) {
			targetUserID = reqOwner
		}
	}

	// 1. FastPanel-style dedicated user & group per virtualhost
	siteUser := strings.TrimSpace(r.FormValue("site_user"))
	if siteUser == "" {
		siteUser = system.GenerateSiteUsername(domainName)
	}
	siteGroup := siteUser

	userHome := filepath.Join(h.cfg.WebRoot, siteUser)

	// Automatically create dedicated Linux user & group (FastPanel style)
	_ = system.CreateSiteUser(r.Context(), siteUser, siteGroup, userHome)

	// 2. Setup user's dedicated directory and site storage inside user's directory:
	// /var/www/<site_user>/data/www/<domain_name>
	// 3. Default folder permissions 0755, file permissions 0644
	_, docRoot, err := system.SetupVHostDirectory(h.cfg.WebRoot, siteUser, domainName)
	if err != nil {
		http.Redirect(w, r, "/domains?msg="+url.QueryEscape("Gagal membuat direktori situs: "+err.Error())+"&type=error", http.StatusSeeOther)
		return
	}

	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	domainID := hex.EncodeToString(idBytes)

	phpSocket := h.phpMgr.GetSocketPath(phpVer)

	domain := model.Domain{
		ID:             domainID,
		UserID:         targetUserID,
		DomainName:     domainName,
		DocumentRoot:   docRoot,
		WebServerType:  wsType,
		PHPVersion:     phpVer,
		PHPSocket:      phpSocket,
		SiteUser:       siteUser,
		SiteGroup:      siteGroup,
		SSLEnabled:     false,
		SSLProvider:    model.SSLProviderNone,
		IsReverseProxy: mode == "proxy",
		ProxyTarget:    proxyTarget,
		IsStatic:       mode == "static",
		Status:         "active",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Create dedicated PHP-FPM pool for this site user if in PHP mode
	if mode != "proxy" && mode != "static" && phpVer != "" {
		if customSock, err := h.phpMgr.ConfigureDomainPool(r.Context(), &domain, siteUser, nil); err == nil && customSock != "" {
			domain.PHPSocket = customSock
		}
	}

	// Generate VHost on the chosen WebServer engine
	_ = h.syncDomainVHosts(r.Context(), &domain)

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

	http.Redirect(w, r, "/domains?msg=Virtual+host+berhasil+dibuat+(User:+"+siteUser+")&type=success", http.StatusSeeOther)
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

	sess := GetSession(r)
	if !IsUserAccessible(sess, h.repo, domain.UserID) {
		h.respondJSON(w, false, "Akses ditolak: Anda tidak memiliki izin untuk domain ini")
		return
	}

	if domain.WebServerType == newEngine {
		h.respondJSON(w, true, "Web engine tidak berubah")
		return
	}

	// Remove VHost from old webserver
	h.removeDomainVHosts(r.Context(), domain, domain.WebServerType)

	// Update domain model
	oldType := domain.WebServerType
	domain.WebServerType = newEngine
	domain.UpdatedAt = time.Now()

	// Generate VHost on new webserver
	if err := h.syncDomainVHosts(r.Context(), domain); err != nil {
		h.respondJSON(w, false, err.Error())
		return
	}

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

	sess := GetSession(r)
	if !IsUserAccessible(sess, h.repo, domain.UserID) {
		h.respondJSON(w, false, "Akses ditolak: Anda tidak memiliki izin untuk domain ini")
		return
	}

	domain.PHPVersion = phpVer
	domain.PHPSocket = h.phpMgr.GetSocketPath(phpVer)

	// Reconfigure isolated PHP pool for site user if configured
	if domain.SiteUser != "" && !domain.IsReverseProxy && !domain.IsStatic {
		if customSock, err := h.phpMgr.ConfigureDomainPool(r.Context(), domain, domain.SiteUser, nil); err == nil && customSock != "" {
			domain.PHPSocket = customSock
		}
	}

	domain.UpdatedAt = time.Now()

	// Regenerate VHost on current webserver
	_ = h.syncDomainVHosts(r.Context(), domain)

	_ = h.repo.UpsertDomain(*domain)

	msg := fmt.Sprintf("Versi PHP untuk %s berhasil diubah ke PHP %s!", domain.DomainName, phpVer)
	h.respondJSON(w, true, msg)
}

// FixPermissions restores FastPanel standard permissions (folders 755, files 644) and ownership
func (h *DomainHandler) FixPermissions(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respondJSON(w, false, "Invalid request form")
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
		if err != nil {
			h.respondJSON(w, false, "Domain tidak ditemukan")
			return
		}
	}

	sess := GetSession(r)
	if !IsUserAccessible(sess, h.repo, domain.UserID) {
		h.respondJSON(w, false, "Akses ditolak: Anda tidak memiliki izin untuk domain ini")
		return
	}

	targetDir := domain.DocumentRoot
	if domain.SiteUser != "" {
		userHome := filepath.Join(h.cfg.WebRoot, domain.SiteUser)
		if fi, err := os.Stat(userHome); err == nil && fi.IsDir() {
			targetDir = userHome
		}
	}

	if err := system.SetVHostPermissions(targetDir, domain.SiteUser, domain.SiteGroup); err != nil {
		h.respondJSON(w, false, "Gagal mereset izin: "+err.Error())
		return
	}

	h.respondJSON(w, true, fmt.Sprintf("Izin folder (755) & file (644) untuk %s berhasil dipulihkan!", domain.DomainName))
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

	sess := GetSession(r)
	if !IsUserAccessible(sess, h.repo, domain.UserID) {
		http.Redirect(w, r, "/domains?msg=Akses+ditolak:+Anda+tidak+memiliki+izin+untuk+domain+ini&type=error", http.StatusSeeOther)
		return
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
		h.removeDomainVHosts(r.Context(), domain, oldWsType)
	}

	// Generate on active engine
	_ = h.syncDomainVHosts(r.Context(), domain)

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

	sess := GetSession(r)
	if !IsUserAccessible(sess, h.repo, domain.UserID) {
		http.Error(w, "Akses ditolak: Anda tidak memiliki izin untuk melihat log domain ini", http.StatusForbidden)
		return
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
			logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_error.log")
			if _, err := os.Stat(logPath); err != nil {
				logPath = filepath.Join("/var/log/nginx", domain.DomainName+".error.log")
			}
		} else {
			logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_access.log")
			if _, err := os.Stat(logPath); err != nil {
				logPath = filepath.Join("/var/log/nginx", domain.DomainName+".access.log")
			}
		}
	}

	content := readLastLines(logPath, 100)
	if strings.TrimSpace(content) == "" {
		if _, err := os.Stat(logPath); err != nil {
			content = fmt.Sprintf("[Berkas log belum terbuat di %s. Log akan otomatis terisi saat ada kunjungan atau request ke domain ini]", logPath)
		} else {
			content = fmt.Sprintf("[%s Log kosong atau belum ada entri terbaru tercatat di %s]", strings.ToUpper(logType), logPath)
		}
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

// ClearLogs clears the specified log file for a virtual host
func (h *DomainHandler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.FormValue("id"))
	logType := strings.TrimSpace(r.FormValue("type"))
	if logType == "" {
		logType = "access"
	}

	domain, err := h.repo.GetDomainByID(id)
	if err != nil {
		domain, err = h.repo.GetDomainByName(id)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Domain tidak ditemukan"})
			return
		}
	}

	sess := GetSession(r)
	if !IsUserAccessible(sess, h.repo, domain.UserID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Akses ditolak"})
		return
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
			logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_error.log")
		} else {
			logPath = filepath.Join("/var/log/apache2", domain.DomainName+"_access.log")
		}
	}

	if logPath != "" {
		_ = os.WriteFile(logPath, []byte(""), 0644)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Log %s untuk %s berhasil dikosongkan.", strings.ToUpper(logType), domain.DomainName),
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
		sess := GetSession(r)
		if !IsUserAccessible(sess, h.repo, domain.UserID) {
			http.Redirect(w, r, "/domains?msg=Akses+ditolak:+Anda+tidak+memiliki+izin+untuk+menghapus+domain+ini&type=error", http.StatusSeeOther)
			return
		}

		h.removeDomainVHosts(r.Context(), domain, domain.WebServerType)
		_ = h.repo.DeleteDomain(domain.ID)
		http.Redirect(w, r, fmt.Sprintf("/domains?msg=Virtual+host+%s+berhasil+dihapus&type=success", domain.DomainName), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/domains?msg=Domain+tidak+ditemukan&type=error", http.StatusSeeOther)
}

// syncDomainVHosts generates and reloads webserver configuration for a virtual host.
// For Apache2 (Option 2), it provisions the Apache backend on 127.0.0.1:8080 (supporting .htaccess & PHP-FPM)
// and the Nginx frontend reverse proxy on Port 80 / 443 (handling SSL & ACME challenges).
func (h *DomainHandler) syncDomainVHosts(ctx context.Context, domain *model.Domain) error {
	if domain.WebServerType == model.WebServerApache {
		// Ensure Apache2 service is active
		_ = h.sm.Start(ctx, "apache2")

		// 1. Generate Apache backend vhost
		if apacheWs, err := h.registry.Get(model.WebServerApache); err == nil {
			if err := apacheWs.GenerateVHost(ctx, domain); err != nil {
				return fmt.Errorf("gagal membuat vhost Apache: %w", err)
			}
			_ = apacheWs.Reload(ctx)
		}

		// 2. Generate Nginx frontend reverse proxy vhost
		if nginxWs, err := h.registry.Get(model.WebServerNginx); err == nil {
			if err := nginxWs.GenerateVHost(ctx, domain); err != nil {
				return fmt.Errorf("gagal membuat reverse proxy Nginx: %w", err)
			}
			_ = nginxWs.Reload(ctx)
		}
		return nil
	}

	// For Nginx standalone or other engines:
	// If switching away from Apache, ensure Apache backend vhost is removed
	if apacheWs, err := h.registry.Get(model.WebServerApache); err == nil {
		_ = apacheWs.RemoveVHost(ctx, domain)
		_ = apacheWs.Reload(ctx)
	}

	ws, err := h.registry.Get(domain.WebServerType)
	if err != nil {
		return fmt.Errorf("webserver engine '%s' tidak didukung: %w", domain.WebServerType, err)
	}

	if err := ws.GenerateVHost(ctx, domain); err != nil {
		return fmt.Errorf("gagal membuat vhost %s: %w", domain.WebServerType, err)
	}
	_ = ws.Reload(ctx)
	return nil
}

func (h *DomainHandler) removeDomainVHosts(ctx context.Context, domain *model.Domain, wsType model.WebServerType) {
	if wsType == model.WebServerApache {
		if apacheWs, err := h.registry.Get(model.WebServerApache); err == nil {
			_ = apacheWs.RemoveVHost(ctx, domain)
			_ = apacheWs.Reload(ctx)
		}
		if nginxWs, err := h.registry.Get(model.WebServerNginx); err == nil {
			_ = nginxWs.RemoveVHost(ctx, domain)
			_ = nginxWs.Reload(ctx)
		}
		return
	}

	if ws, err := h.registry.Get(wsType); err == nil {
		_ = ws.RemoveVHost(ctx, domain)
		_ = ws.Reload(ctx)
	}
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
