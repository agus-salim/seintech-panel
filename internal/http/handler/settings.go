package handler

import (
	"net/http"
	"strconv"

	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type SettingsHandler struct {
	repo *storage.Repository
	sm   *system.ServiceManager
}

func NewSettingsHandler(repo *storage.Repository, sm *system.ServiceManager) *SettingsHandler {
	return &SettingsHandler{repo: repo, sm: sm}
}

type SettingsPageData struct {
	BasePageData
	Settings *model.PanelSettings
	Services []system.ServiceState
}

func (h *SettingsHandler) View(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	ctx := r.Context()

	serviceNames := []string{"nginx", "apache2", "caddy", "mariadb", "mysql", "postgresql", "php8.3-fpm"}
	var services []system.ServiceState
	for _, s := range serviceNames {
		st, _ := h.sm.GetStatus(ctx, s)
		services = append(services, st)
	}

	data := SettingsPageData{
		BasePageData: NewBaseData(r, "Settings", "Panel & Server Configuration", "settings", settings.Hostname),
		Settings:     settings,
		Services:     services,
	}

	_ = web.RenderTemplate(w, "settings.html", data)
}

func (h *SettingsHandler) Save(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()

	settings.Hostname = r.FormValue("hostname")
	if p, err := strconv.Atoi(r.FormValue("port")); err == nil {
		settings.Port = p
	}
	settings.DefaultWebServer = r.FormValue("default_webserver")
	settings.DefaultPHPVersion = r.FormValue("default_php_version")
	settings.SecurityHeadersEnabled = r.FormValue("security_headers") == "true"

	_ = h.repo.SaveSettings(*settings)

	http.Redirect(w, r, "/settings?success=saved", http.StatusSeeOther)
}

func (h *SettingsHandler) RestartService(w http.ResponseWriter, r *http.Request) {
	service := r.FormValue("service")
	if service != "" {
		_ = h.sm.Restart(r.Context(), service)
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}
