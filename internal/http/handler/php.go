package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

func (h *PHPHandler) GetExtensions(w http.ResponseWriter, r *http.Request) {
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	if version == "" {
		version = "8.3"
	}

	exts, err := h.phpMgr.GetExtensions(r.Context(), version)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"version":    version,
		"extensions": exts,
	})
}

func (h *PHPHandler) InstallExtension(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	version := strings.TrimSpace(r.FormValue("version"))
	ext := strings.TrimSpace(r.FormValue("extension"))

	if version == "" || ext == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Parameter versi dan ekstensi harus diisi",
		})
		return
	}

	err := h.phpMgr.InstallExtension(r.Context(), version, ext)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	msg := fmt.Sprintf("Ekstensi php%s-%s berhasil dipasang!", version, ext)
	if ext == "ioncube" || ext == "ioncube-loader" {
		msg = fmt.Sprintf("Ekstensi ionCube Loader untuk PHP %s berhasil dipasang!", version)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": msg,
	})
}

func (h *PHPHandler) DeleteExtension(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	version := strings.TrimSpace(r.FormValue("version"))
	ext := strings.TrimSpace(r.FormValue("extension"))

	if version == "" || ext == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Parameter versi dan ekstensi harus diisi",
		})
		return
	}

	err := h.phpMgr.DeleteExtension(r.Context(), version, ext)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	msg := fmt.Sprintf("Ekstensi php%s-%s berhasil dihapus!", version, ext)
	if ext == "ioncube" || ext == "ioncube-loader" {
		msg = fmt.Sprintf("Ekstensi ionCube Loader untuk PHP %s berhasil dihapus!", version)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": msg,
	})
}

func (h *PHPHandler) GetIni(w http.ResponseWriter, r *http.Request) {
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	if version == "" {
		version = "8.3"
	}

	res, err := h.phpMgr.GetIni(r.Context(), version)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"version":    res["version"],
		"file":       res["file"],
		"content":    res["content"],
		"directives": res["directives"],
	})
}

func (h *PHPHandler) SaveIni(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	version := strings.TrimSpace(r.FormValue("version"))
	mode := strings.TrimSpace(r.FormValue("mode")) // "raw" or "directives"

	if version == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Parameter versi PHP harus diisi",
		})
		return
	}

	var err error
	if mode == "raw" {
		content := r.FormValue("content")
		err = h.phpMgr.SaveIniRaw(r.Context(), version, content)
	} else {
		updates := make(map[string]string)
		for _, d := range php.CommonDirectives {
			if val := r.FormValue(d); val != "" {
				updates[d] = val
			}
		}
		err = h.phpMgr.SaveIniDirectives(r.Context(), version, updates)
	}

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Konfigurasi php.ini PHP %s berhasil disimpan & PHP-FPM di-restart!", version),
	})
}
