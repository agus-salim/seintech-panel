package handler

import (
	"net/http"

	"github.com/seintech/panel/internal/core/php"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type PHPHandler struct {
	repo   *storage.Repository
	phpMgr *php.PHPManager
}

func NewPHPHandler(repo *storage.Repository, phpMgr *php.PHPManager) *PHPHandler {
	return &PHPHandler{repo: repo, phpMgr: phpMgr}
}

type PHPPageData struct {
	BasePageData
	Versions []php.PHPVersionInfo
}

func (h *PHPHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	versions, _ := h.phpMgr.ListVersions(r.Context())

	data := PHPPageData{
		BasePageData: NewBaseData(r, "PHP Engines", "Multi-PHP Version Engine & FPM Pools", "php", settings.Hostname),
		Versions:     versions,
	}

	_ = web.RenderTemplate(w, "php.html", data)
}
