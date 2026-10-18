package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type ServicesHandler struct {
	repo   *storage.Repository
	sm     *system.ServiceManager
	sslMgr *ssl.SSLManager
}

func NewServicesHandler(repo *storage.Repository, sm *system.ServiceManager, sslMgr *ssl.SSLManager) *ServicesHandler {
	return &ServicesHandler{repo: repo, sm: sm, sslMgr: sslMgr}
}

type ServicesPageData struct {
	BasePageData
	WebServers        []system.ServiceDetail
	Databases         []system.ServiceDetail
	DatabaseTools     []system.ServiceDetail
	PHPEngines        []system.ServiceDetail
	AvailableServices []system.ServiceDetail
}

func (h *ServicesHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	all := h.sm.GetAllServicesList(r.Context())

	var webservers, databases, databaseTools, phps, available []system.ServiceDetail
	for _, s := range all {
		if s.Installed {
			switch s.Category {
			case "webserver":
				webservers = append(webservers, s)
			case "database":
				databases = append(databases, s)
			case "database_tool":
				databaseTools = append(databaseTools, s)
			case "php":
				phps = append(phps, s)
			}
		} else {
			available = append(available, s)
		}
	}

	flash := r.URL.Query().Get("msg")
	flashType := r.URL.Query().Get("type")
	if flashType == "" {
		flashType = "info"
	}

	base := NewBaseData(r, "Services & Packages", "Service Manager & Package Installer", "services", settings.Hostname, settings.Language)
	base.FlashMessage = flash
	base.FlashType = flashType

	data := ServicesPageData{
		BasePageData:      base,
		WebServers:        webservers,
		Databases:         databases,
		DatabaseTools:     databaseTools,
		PHPEngines:        phps,
		AvailableServices: available,
	}

	_ = web.RenderTemplate(w, "services.html", data)
}

// ExecuteAction handles quick synchronous actions: start, stop, restart, reload.
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
	default:
		err = fmt.Errorf("unknown action: %s", action)
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			msg = fmt.Sprintf("Service %s belum terpasang di sistem (unit service tidak ditemukan). Silakan klik tombol Install.", service)
		} else {
			msg = fmt.Sprintf("Error performing %s on %s: %v", action, service, err)
		}
		msgType = "error"
	} else if service == "phpmyadmin" || service == "phppgadmin" || strings.HasPrefix(service, "php") {
		if h.sslMgr != nil {
			_ = h.sslMgr.SyncPanelNginxReverseProxy(ctx)
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/services?msg=%s&type=%s", url.QueryEscape(msg), msgType), http.StatusSeeOther)
}

// StartInstallJob kicks off an async install job and returns the job ID as JSON.
func (h *ServicesHandler) StartInstallJob(w http.ResponseWriter, r *http.Request) {
	service := r.FormValue("service")
	if service == "" {
		http.Error(w, "missing service", http.StatusBadRequest)
		return
	}

	js := system.GetJobStore()
	job := js.Create(service, "install")

	// Run in background goroutine
	go func() {
		h.sm.RunInstallJob(job.ID, service)
		if h.sslMgr != nil && (service == "phpmyadmin" || service == "phppgadmin" || strings.HasPrefix(service, "php")) {
			_ = h.sslMgr.SyncPanelNginxReverseProxy(context.Background())
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID})
}

// StartUninstallJob kicks off an async uninstall job and returns the job ID as JSON.
func (h *ServicesHandler) StartUninstallJob(w http.ResponseWriter, r *http.Request) {
	service := r.FormValue("service")
	if service == "" {
		http.Error(w, "missing service", http.StatusBadRequest)
		return
	}

	js := system.GetJobStore()
	job := js.Create(service, "uninstall")

	// Run in background goroutine
	go func() {
		h.sm.RunUninstallJob(job.ID, service)
		if h.sslMgr != nil && (service == "phpmyadmin" || service == "phppgadmin" || strings.HasPrefix(service, "php")) {
			_ = h.sslMgr.SyncPanelNginxReverseProxy(context.Background())
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID})
}

// GetJobStatus returns the current state of a background job as JSON.
func (h *ServicesHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	js := system.GetJobStore()
	job, ok := js.Get(jobID)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}
