package handler

import (
	"net/http"

	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type DashboardHandler struct {
	repo *storage.Repository
	sm   *system.ServiceManager
}

func NewDashboardHandler(repo *storage.Repository, sm *system.ServiceManager) *DashboardHandler {
	return &DashboardHandler{repo: repo, sm: sm}
}

type DashboardData struct {
	BasePageData
	Metrics        system.SystemMetrics
	Services       []system.ServiceState
	Domains        []model.Domain
	ShowOnboarding bool
	HasWebserver   bool
	HasPHP         bool
	HasDatabase    bool
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, _ := h.repo.GetSettings()
	domains, _ := h.repo.GetDomains()

	metrics := system.CollectMetrics()

	// Check core services status
	serviceNames := []string{"nginx", "apache2", "mariadb", "postgresql", "php8.3-fpm"}
	var services []system.ServiceState
	for _, s := range serviceNames {
		st, _ := h.sm.GetStatus(ctx, s)
		services = append(services, st)
	}

	// Check installed stacks for onboarding check
	all := h.sm.GetAllServicesList(ctx)
	hasWebserver := false
	hasPHP := false
	hasDatabase := false
	for _, s := range all {
		if s.Installed {
			switch s.Category {
			case "webserver":
				hasWebserver = true
			case "php":
				hasPHP = true
			case "database":
				hasDatabase = true
			}
		}
	}

	data := DashboardData{
		BasePageData:   NewBaseData(r, "Dashboard", "System Overview & Node Metrics", "dashboard", settings.Hostname, settings.Language),
		Metrics:        metrics,
		Services:       services,
		Domains:        domains,
		ShowOnboarding: !hasWebserver,
		HasWebserver:   hasWebserver,
		HasPHP:         hasPHP,
		HasDatabase:    hasDatabase,
	}

	_ = web.RenderTemplate(w, "dashboard.html", data)
}
