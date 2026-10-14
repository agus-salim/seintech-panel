package handler

import (
	"fmt"
	"net/http"

	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type ServicesHandler struct {
	repo *storage.Repository
	sm   *system.ServiceManager
}

func NewServicesHandler(repo *storage.Repository, sm *system.ServiceManager) *ServicesHandler {
	return &ServicesHandler{repo: repo, sm: sm}
}

type ServicesPageData struct {
	BasePageData
	WebServers []system.ServiceDetail
	Databases  []system.ServiceDetail
	PHPEngines []system.ServiceDetail
}

func (h *ServicesHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	all := h.sm.GetAllServicesList(r.Context())

	var webservers, databases, phps []system.ServiceDetail
	for _, s := range all {
		switch s.Category {
		case "webserver":
			webservers = append(webservers, s)
		case "database":
			databases = append(databases, s)
		case "php":
			phps = append(phps, s)
		}
	}

	flash := r.URL.Query().Get("msg")
	flashType := r.URL.Query().Get("type")
	if flashType == "" {
		flashType = "info"
	}

	base := NewBaseData(r, "Services & Packages", "Service Manager & Package Installer", "services", settings.Hostname)
	base.FlashMessage = flash
	base.FlashType = flashType

	data := ServicesPageData{
		BasePageData: base,
		WebServers:   webservers,
		Databases:    databases,
		PHPEngines:   phps,
	}

	_ = web.RenderTemplate(w, "services.html", data)
}

func (h *ServicesHandler) ExecuteAction(w http.ResponseWriter, r *http.Request) {
	service := r.FormValue("service")
	action := r.FormValue("action")
	ctx := r.Context()

	var err error
	msg := ""
	msgType := "success"

	switch action {
	case "start":
		err = h.sm.Start(ctx, service)
		msg = fmt.Sprintf("Service %s started successfully", service)
	case "stop":
		err = h.sm.Stop(ctx, service)
		msg = fmt.Sprintf("Service %s stopped successfully", service)
	case "restart":
		err = h.sm.Restart(ctx, service)
		msg = fmt.Sprintf("Service %s restarted successfully", service)
	case "reload":
		err = h.sm.Reload(ctx, service)
		msg = fmt.Sprintf("Service %s reloaded successfully", service)
	case "install":
		err = h.sm.InstallService(ctx, service)
		msg = fmt.Sprintf("Service %s installed and started successfully", service)
	case "uninstall":
		err = h.sm.UninstallService(ctx, service)
		msg = fmt.Sprintf("Service %s has been removed completely", service)
	default:
		err = fmt.Errorf("unknown action: %s", action)
	}

	if err != nil {
		msg = fmt.Sprintf("Error performing %s on %s: %v", action, service, err)
		msgType = "error"
	}

	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/services"
	}

	http.Redirect(w, r, fmt.Sprintf("/services?msg=%s&type=%s", msg, msgType), http.StatusSeeOther)
}
