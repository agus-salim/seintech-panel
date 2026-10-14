package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type SettingsHandler struct {
	repo   *storage.Repository
	sm     *system.ServiceManager
	sslMgr *ssl.SSLManager
}

func NewSettingsHandler(repo *storage.Repository, sm *system.ServiceManager, sslMgr *ssl.SSLManager) *SettingsHandler {
	return &SettingsHandler{repo: repo, sm: sm, sslMgr: sslMgr}
}

type SettingsPageData struct {
	BasePageData
	Settings *model.PanelSettings
}

func (h *SettingsHandler) View(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()

	// Ensure hosts slice is initialized
	if len(settings.Hosts) == 0 {
		primaryHost := settings.Hostname
		if primaryHost == "" {
			primaryHost = "cp.ppdmalfalah.id"
		}
		primaryPort := settings.Port
		if primaryPort == 0 {
			primaryPort = 8443
		}
		settings.Hosts = []model.PanelHost{
			{
				ID:          "host-primary",
				Hostname:    primaryHost,
				Port:        primaryPort,
				SSLEnabled:  settings.SSLEnabled,
				SSLProvider: "letsencrypt",
				IsPrimary:   true,
			},
		}
		_ = h.repo.SaveSettings(*settings)
	}

	// Synchronize authentic SSL certificates on disk for all panel hosts
	for i := range settings.Hosts {
		status := h.sslMgr.GetPanelHostSSLStatus(settings.Hosts[i].Hostname)
		settings.Hosts[i].SSLEnabled = status.IsActive
		settings.Hosts[i].SSLExpiry = status.ExpiresAt
		if status.IsLetsEncrypt {
			settings.Hosts[i].SSLProvider = "letsencrypt"
		}
		if settings.Hosts[i].IsPrimary {
			settings.SSLEnabled = status.IsActive
		}
	}

	data := SettingsPageData{
		BasePageData: NewBaseData(r, "Panel Settings", "Panel & Server Configuration", "settings", settings.Hostname),
		Settings:     settings,
	}

	_ = web.RenderTemplate(w, "settings.html", data)
}

func (h *SettingsHandler) Save(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()

	settings.DefaultWebServer = r.FormValue("default_webserver")
	settings.DefaultPHPVersion = r.FormValue("default_php_version")
	settings.SecurityHeadersEnabled = r.FormValue("security_headers") == "true"

	_ = h.repo.SaveSettings(*settings)

	http.Redirect(w, r, "/settings?msg=Konfigurasi+server+berhasil+disimpan&type=success", http.StatusSeeOther)
}

func (h *SettingsHandler) AddHost(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()

	hostname := cleanHostname(r.FormValue("hostname"))
	if hostname == "" {
		http.Redirect(w, r, "/settings?msg=Alamat+domain+panel+tidak+boleh+kosong&type=error", http.StatusSeeOther)
		return
	}

	port := 8443
	if p, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port"))); err == nil && p > 0 {
		port = p
	}

	// Check duplicate
	for _, host := range settings.Hosts {
		if strings.EqualFold(host.Hostname, hostname) {
			http.Redirect(w, r, "/settings?msg=Domain+panel+tersebut+sudah+terdaftar&type=error", http.StatusSeeOther)
			return
		}
	}

	isPrimary := len(settings.Hosts) == 0
	newHost := model.PanelHost{
		ID:          fmt.Sprintf("host-%d", time.Now().UnixNano()),
		Hostname:    hostname,
		Port:        port,
		SSLEnabled:  false,
		SSLProvider: "letsencrypt",
		IsPrimary:   isPrimary,
	}

	settings.Hosts = append(settings.Hosts, newHost)
	_ = h.repo.SaveSettings(*settings)
	_ = h.sslMgr.SyncPanelNginxReverseProxy(r.Context())

	http.Redirect(w, r, "/settings?msg=Domain+panel+berhasil+ditambahkan&type=success", http.StatusSeeOther)
}

func (h *SettingsHandler) UpdateHost(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()

	hostID := strings.TrimSpace(r.FormValue("host_id"))
	hostname := cleanHostname(r.FormValue("hostname"))
	if hostname == "" {
		http.Redirect(w, r, "/settings?msg=Alamat+domain+panel+tidak+boleh+kosong&type=error", http.StatusSeeOther)
		return
	}

	port := 8443
	if p, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port"))); err == nil && p > 0 {
		port = p
	}

	found := false
	for i := range settings.Hosts {
		if settings.Hosts[i].ID == hostID {
			settings.Hosts[i].Hostname = hostname
			settings.Hosts[i].Port = port
			if settings.Hosts[i].IsPrimary {
				settings.Hostname = hostname
				settings.Port = port
			}
			found = true
			break
		}
	}

	if !found {
		http.Redirect(w, r, "/settings?msg=Domain+panel+tidak+ditemukan&type=error", http.StatusSeeOther)
		return
	}

	_ = h.repo.SaveSettings(*settings)
	_ = h.sslMgr.SyncPanelNginxReverseProxy(r.Context())
	http.Redirect(w, r, "/settings?msg=Domain+panel+berhasil+diperbarui&type=success", http.StatusSeeOther)
}

func (h *SettingsHandler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	hostID := strings.TrimSpace(r.FormValue("host_id"))

	if len(settings.Hosts) <= 1 {
		http.Redirect(w, r, "/settings?msg=Minimal+harus+tersisa+satu+domain+akses+panel&type=error", http.StatusSeeOther)
		return
	}

	var updated []model.PanelHost
	for _, host := range settings.Hosts {
		if host.ID == hostID {
			if host.IsPrimary {
				http.Redirect(w, r, "/settings?msg=Domain+utama+panel+tidak+dapat+dihapus&type=error", http.StatusSeeOther)
				return
			}
			continue
		}
		updated = append(updated, host)
	}

	settings.Hosts = updated
	_ = h.repo.SaveSettings(*settings)
	_ = h.sslMgr.SyncPanelNginxReverseProxy(r.Context())
	http.Redirect(w, r, "/settings?msg=Domain+panel+berhasil+dihapus&type=success", http.StatusSeeOther)
}

func (h *SettingsHandler) IssueSSL(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	hostID := strings.TrimSpace(r.FormValue("host_id"))
	hostname := cleanHostname(r.FormValue("hostname"))

	var targetHost *model.PanelHost
	for i := range settings.Hosts {
		if (hostID != "" && settings.Hosts[i].ID == hostID) ||
			(hostname != "" && strings.EqualFold(settings.Hosts[i].Hostname, hostname)) {
			targetHost = &settings.Hosts[i]
			break
		}
	}

	isAJAX := r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json")

	if targetHost == nil {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Domain panel tidak ditemukan"})
			return
		}
		http.Redirect(w, r, "/settings?msg=Domain+panel+tidak+ditemukan&type=error", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	expiry, err := h.sslMgr.ProvisionPanelHostLetsEncrypt(ctx, targetHost.Hostname)
	if err != nil {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/settings?msg=Gagal+mengaktifkan+SSL:+%s&type=error", err.Error()), http.StatusSeeOther)
		return
	}

	targetHost.SSLEnabled = true
	targetHost.SSLProvider = "letsencrypt"
	targetHost.SSLExpiry = expiry
	if targetHost.IsPrimary {
		settings.SSLEnabled = true
	}
	_ = h.repo.SaveSettings(*settings)
	_ = h.sslMgr.SyncPanelNginxReverseProxy(ctx)

	msg := fmt.Sprintf("Sertifikat SSL Let's Encrypt untuk %s berhasil diaktifkan!", targetHost.Hostname)
	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": msg})
		return
	}
	http.Redirect(w, r, "/settings?msg="+msg+"&type=success", http.StatusSeeOther)
}

func (h *SettingsHandler) SyncProxy(w http.ResponseWriter, r *http.Request) {
	if err := h.sslMgr.SyncPanelNginxReverseProxy(r.Context()); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/settings?msg=Gagal+sinkronisasi+reverse+proxy:+%s&type=error", err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?msg=Reverse+proxy+Nginx+berhasil+disinkronkan+(Port+443)&type=success", http.StatusSeeOther)
}

func cleanHostname(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":"); idx != -1 {
		s = s[:idx]
	}
	return strings.ToLower(strings.TrimSpace(s))
}
