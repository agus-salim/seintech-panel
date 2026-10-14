package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

	// Authentically verify each domain's SSL certificate on disk
	for i := range domains {
		_ = h.sslMgr.SyncDomainSSLStatus(&domains[i])
	}

	data := SSLPageData{
		BasePageData: NewBaseData(r, "SSL & Certificates", "Automated Let's Encrypt & Custom SSL", "ssl", settings.Hostname),
		Domains:      domains,
	}

	_ = web.RenderTemplate(w, "ssl.html", data)
}

func (h *SSLHandler) Issue(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.respond(w, r, false, "Invalid request form data")
		return
	}

	domainID := r.FormValue("domain_id")
	domainName := r.FormValue("domain_name")

	domains, _ := h.repo.GetDomains()
	var targetDomain *model.Domain
	for i := range domains {
		if domains[i].ID == domainID || (domainName != "" && domains[i].DomainName == domainName) {
			targetDomain = &domains[i]
			break
		}
	}

	if targetDomain == nil {
		h.respond(w, r, false, "Domain tidak ditemukan")
		return
	}

	err := h.sslMgr.ProvisionLetsEncrypt(r.Context(), targetDomain)
	if err != nil {
		h.respond(w, r, false, "Gagal mengaktifkan Let's Encrypt: "+err.Error())
		return
	}

	msg := fmt.Sprintf("Sertifikat SSL Let's Encrypt untuk %s berhasil diaktifkan dan aktif!", targetDomain.DomainName)
	h.respond(w, r, true, msg)
}

func (h *SSLHandler) respond(w http.ResponseWriter, r *http.Request, success bool, message string) {
	isAJAX := r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json") ||
		r.URL.Query().Get("json") == "1"

	if isAJAX {
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
		return
	}

	msgType := "success"
	if !success {
		msgType = "error"
	}
	redirectURL := r.Header.Get("Referer")
	if redirectURL == "" {
		redirectURL = "/ssl"
	}
	cleanURL, _, _ := strings.Cut(redirectURL, "?")
	http.Redirect(w, r, fmt.Sprintf("%s?msg=%s&type=%s", cleanURL, url.QueryEscape(message), msgType), http.StatusSeeOther)
}

