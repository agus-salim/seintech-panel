package handler

import (
	"net/http"

	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type SSLHandler struct {
	repo   *storage.Repository
	sslMgr *ssl.SSLManager
}

func NewSSLHandler(repo *storage.Repository, sslMgr *ssl.SSLManager) *SSLHandler {
	return &SSLHandler{repo: repo, sslMgr: sslMgr}
}

type SSLPageData struct {
	BasePageData
	Domains []model.Domain
}

func (h *SSLHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.repo.GetSettings()
	domains, _ := h.repo.GetDomains()

	data := SSLPageData{
		BasePageData: NewBaseData(r, "SSL & Certificates", "Automated Let's Encrypt & Custom SSL", "ssl", settings.Hostname),
		Domains:      domains,
	}

	_ = web.RenderTemplate(w, "ssl.html", data)
}

func (h *SSLHandler) Issue(w http.ResponseWriter, r *http.Request) {
	domainID := r.FormValue("domain_id")
	domains, _ := h.repo.GetDomains()

	for _, d := range domains {
		if d.ID == domainID {
			_ = h.sslMgr.ProvisionLetsEncrypt(r.Context(), &d)
			break
		}
	}

	http.Redirect(w, r, "/ssl?success=issued", http.StatusSeeOther)
}
