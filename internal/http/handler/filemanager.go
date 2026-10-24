package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/filemanager"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
	"github.com/seintech/panel/web"
)

type FileManagerHandler struct {
	repo *storage.Repository
	cfg  *config.Config
}

func NewFileManagerHandler(repo *storage.Repository, cfg *config.Config) *FileManagerHandler {
	return &FileManagerHandler{repo: repo, cfg: cfg}
}

type Breadcrumb struct {
	Name string
	Path string
}

type FMPageData struct {
	BasePageData
	CurrentPath     string
	ParentPath      string
	Breadcrumbs     []Breadcrumb
	Files           []filemanager.FileItem
	ActiveSite      string
	AccessibleSites []model.Domain
}

func (h *FileManagerHandler) getFMForUser(r *http.Request) (*filemanager.FileManager, string, []model.Domain, error) {
	sess := GetSession(r)
	accessibleIDs := GetAccessibleUserIDs(sess, h.repo)

	allDomains, _ := h.repo.GetDomains()
	var accessibleDomains []model.Domain
	for _, d := range allDomains {
		if accessibleIDs == nil || accessibleIDs[d.UserID] {
			accessibleDomains = append(accessibleDomains, d)
		}
	}

	reqSite := strings.TrimSpace(r.URL.Query().Get("site"))
	if reqSite == "" {
		reqSite = strings.TrimSpace(r.FormValue("site"))
	}

	jailRoot := h.cfg.WebRoot
	activeSite := ""

	if sess.Role == model.RoleUser {
		// End user is strictly jailed to their own 1 site's DocumentRoot
		if len(accessibleDomains) > 0 {
			target := accessibleDomains[0]
			activeSite = target.DomainName
			if target.DocumentRoot != "" {
				if _, err := os.Stat(target.DocumentRoot); err == nil {
					jailRoot = target.DocumentRoot
				} else if target.SiteUser != "" {
					siteHome := filepath.Join(h.cfg.WebRoot, target.SiteUser)
					if _, err := os.Stat(siteHome); err == nil {
						jailRoot = siteHome
					}
				}
			}
		} else {
			jailRoot = filepath.Join(h.cfg.WebRoot, sess.Username)
		}
	} else if sess.Role == model.RoleReseller {
		// Reseller is jailed to the sites they created or their end-users' sites
		var chosenDomain *model.Domain
		for i := range accessibleDomains {
			if reqSite != "" && strings.EqualFold(accessibleDomains[i].DomainName, reqSite) {
				chosenDomain = &accessibleDomains[i]
				break
			}
		}
		if chosenDomain == nil && len(accessibleDomains) > 0 {
			chosenDomain = &accessibleDomains[0]
		}

		if chosenDomain != nil {
			activeSite = chosenDomain.DomainName
			if chosenDomain.DocumentRoot != "" {
				if _, err := os.Stat(chosenDomain.DocumentRoot); err == nil {
					jailRoot = chosenDomain.DocumentRoot
				} else if chosenDomain.SiteUser != "" {
					siteHome := filepath.Join(h.cfg.WebRoot, chosenDomain.SiteUser)
					if _, err := os.Stat(siteHome); err == nil {
						jailRoot = siteHome
					}
				}
			}
		} else {
			jailRoot = filepath.Join(h.cfg.WebRoot, sess.Username)
		}
	} else {
		// Admin: can optionally switch site or view all /var/www
		if reqSite != "" {
			for i := range accessibleDomains {
				if strings.EqualFold(accessibleDomains[i].DomainName, reqSite) {
					activeSite = accessibleDomains[i].DomainName
					if accessibleDomains[i].DocumentRoot != "" {
						jailRoot = accessibleDomains[i].DocumentRoot
					}
					break
				}
			}
		}
	}

	fm, err := filemanager.NewFileManager(jailRoot)
	return fm, activeSite, accessibleDomains, err
}

func (h *FileManagerHandler) List(w http.ResponseWriter, r *http.Request) {
	fm, activeSite, accessibleSites, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, "File manager initialization error", http.StatusInternalServerError)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	cleanPath := fm.GetRelativePath(path)
	if cleanPath == "" {
		cleanPath = "/"
	}

	items, err := fm.List(cleanPath)
	if err != nil {
		items = []filemanager.FileItem{}
	}

	settings, _ := h.repo.GetSettings()

	// Build breadcrumbs
	parts := strings.Split(strings.Trim(cleanPath, "/"), "/")
	var crumbs []Breadcrumb
	accum := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		accum += "/" + p
		crumbs = append(crumbs, Breadcrumb{Name: p, Path: accum})
	}

	parent := "/"
	if cleanPath != "/" && cleanPath != "." {
		parent = filepath.ToSlash(filepath.Dir(cleanPath))
	}

	data := FMPageData{
		BasePageData:    NewBaseData(r, "File Manager", "Web File Manager & Code Editor", "filemanager", settings.Hostname, settings.Language),
		CurrentPath:     cleanPath,
		ParentPath:      parent,
		Breadcrumbs:     crumbs,
		Files:           items,
		ActiveSite:      activeSite,
		AccessibleSites: accessibleSites,
	}

	_ = web.RenderTemplate(w, "filemanager.html", data)
}

func redirectFMPath(path, site string) string {
	if site != "" {
		return fmt.Sprintf("/filemanager?path=%s&site=%s", url.QueryEscape(path), url.QueryEscape(site))
	}
	return fmt.Sprintf("/filemanager?path=%s", url.QueryEscape(path))
}

func (h *FileManagerHandler) Read(w http.ResponseWriter, r *http.Request) {
	fm, _, _, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.URL.Query().Get("path")
	content, err := fm.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(content))
}

func (h *FileManagerHandler) Save(w http.ResponseWriter, r *http.Request) {
	fm, _, _, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.FormValue("path")
	content := r.FormValue("content")

	if err := fm.WriteFile(path, []byte(content)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("saved"))
}

func (h *FileManagerHandler) Download(w http.ResponseWriter, r *http.Request) {
	fm, _, _, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.URL.Query().Get("path")
	content, err := fm.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write([]byte(content))
}

func (h *FileManagerHandler) Upload(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")

	fm, activeSite, _, err := h.getFMForUser(r)
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Support up to 10GB file uploads with safe 32MB in-RAM buffer, streaming rest to disk
	r.Body = http.MaxBytesReader(w, r.Body, 10<<30)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("[FileManager] Upload ParseMultipartForm error: %v", err)
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Gagal membaca form data: " + err.Error()})
			return
		}
		http.Error(w, "Failed reading uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}

	destPath := r.FormValue("path")
	if destPath == "" {
		destPath = "/"
	}

	var files []*multipart.FileHeader
	if r.MultipartForm != nil {
		if fList, ok := r.MultipartForm.File["file"]; ok && len(fList) > 0 {
			files = append(files, fList...)
		} else if fList, ok := r.MultipartForm.File["files"]; ok {
			files = append(files, fList...)
		} else {
			for _, headers := range r.MultipartForm.File {
				files = append(files, headers...)
			}
		}
	}

	if len(files) == 0 {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Tidak ada file yang diunggah"})
			return
		}
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}

	var uploaded []string
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			log.Printf("[FileManager] Open header error for %s: %v", header.Filename, err)
			continue
		}

		cleanFileName := filepath.Base(filepath.ToSlash(header.Filename))
		if cleanFileName == "." || cleanFileName == "/" || cleanFileName == "" {
			file.Close()
			continue
		}

		target := filepath.Join(destPath, cleanFileName)
		err = fm.WriteStream(target, file)
		file.Close()
		if err != nil {
			log.Printf("[FileManager] WriteStream error for %s (target: %s): %v", header.Filename, target, err)
			if isAjax {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("Gagal menulis file %s: %v", cleanFileName, err)})
				return
			}
			http.Error(w, fmt.Sprintf("Failed writing file %s: %v", cleanFileName, err), http.StatusInternalServerError)
			return
		}
		uploaded = append(uploaded, cleanFileName)
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Berhasil mengunggah %d file", len(uploaded)),
			"files":   uploaded,
		})
		return
	}

	http.Redirect(w, r, redirectFMPath(destPath, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	fm, activeSite, _, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.FormValue("path")
	parent := filepath.ToSlash(filepath.Dir(path))
	_ = fm.Delete(path)

	http.Redirect(w, r, redirectFMPath(parent, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) CreateFile(w http.ResponseWriter, r *http.Request) {
	fm, activeSite, _, _ := h.getFMForUser(r)
	path := r.FormValue("path")
	filename := r.FormValue("filename")
	target := filepath.Join(path, filename)
	_ = fm.WriteFile(target, []byte(""))

	http.Redirect(w, r, redirectFMPath(path, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) CreateDir(w http.ResponseWriter, r *http.Request) {
	fm, activeSite, _, _ := h.getFMForUser(r)
	path := r.FormValue("path")
	dirname := r.FormValue("dirname")
	target := filepath.Join(path, dirname)
	_ = fm.CreateDir(target)

	http.Redirect(w, r, redirectFMPath(path, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) Chmod(w http.ResponseWriter, r *http.Request) {
	fm, activeSite, _, _ := h.getFMForUser(r)
	path := r.FormValue("path")
	modeStr := r.FormValue("mode")

	val, err := strconv.ParseUint(modeStr, 8, 32)
	if err == nil {
		_ = fm.Chmod(path, os.FileMode(val))
	}

	parent := filepath.ToSlash(filepath.Dir(path))
	http.Redirect(w, r, redirectFMPath(parent, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) Rename(w http.ResponseWriter, r *http.Request) {
	fm, activeSite, _, _ := h.getFMForUser(r)
	oldPath := r.FormValue("old_path")
	newName := r.FormValue("new_name")
	parent := filepath.ToSlash(filepath.Dir(oldPath))
	newPath := filepath.Join(parent, newName)
	_ = fm.Rename(oldPath, newPath)

	http.Redirect(w, r, redirectFMPath(parent, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) Extract(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")
	fm, activeSite, _, err := h.getFMForUser(r)
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = r.ParseMultipartForm(32 << 20)
	_ = r.ParseForm()

	path := strings.TrimSpace(r.FormValue("path"))
	dest := strings.TrimSpace(r.FormValue("dest"))
	if dest == "" {
		dest = filepath.ToSlash(filepath.Dir(path))
	}

	if err := fm.ExtractArchive(path, dest); err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, "Gagal mengekstrak: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Arsip berhasil diekstrak"})
		return
	}

	http.Redirect(w, r, redirectFMPath(dest, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) Compress(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")
	fm, activeSite, _, err := h.getFMForUser(r)
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = r.ParseMultipartForm(32 << 20)
	_ = r.ParseForm()

	destDir := strings.TrimSpace(r.FormValue("dest_dir"))
	if destDir == "" {
		destDir = "/"
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "archive.zip"
	}
	lowerName := strings.ToLower(name)
	if !strings.HasSuffix(lowerName, ".zip") && !strings.HasSuffix(lowerName, ".tar.gz") && !strings.HasSuffix(lowerName, ".tgz") && !strings.HasSuffix(lowerName, ".tar") {
		name += ".zip"
	}
	targetZip := filepath.Join(destDir, name)

	var paths []string
	paths = r.Form["paths"]
	if len(paths) == 0 && r.MultipartForm != nil {
		paths = r.MultipartForm.Value["paths"]
	}
	if len(paths) == 0 && r.FormValue("paths_str") != "" {
		paths = strings.Split(r.FormValue("paths_str"), ",")
	}

	if len(paths) == 0 {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Tidak ada file yang dipilih untuk dikompresi"})
			return
		}
		http.Error(w, "Tidak ada file yang dipilih untuk dikompres", http.StatusBadRequest)
		return
	}

	if err := fm.CompressArchive(paths, targetZip); err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, "Gagal mengompres: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "File berhasil dikompres ke " + name})
		return
	}

	http.Redirect(w, r, redirectFMPath(destDir, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")
	fm, activeSite, _, err := h.getFMForUser(r)
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = r.ParseMultipartForm(32 << 20)
	_ = r.ParseForm()

	currentPath := r.FormValue("current_path")
	if currentPath == "" {
		currentPath = "/"
	}

	var paths []string
	paths = r.Form["paths"]
	if len(paths) == 0 && r.MultipartForm != nil {
		paths = r.MultipartForm.Value["paths"]
	}
	if len(paths) == 0 && r.FormValue("paths_str") != "" {
		paths = strings.Split(r.FormValue("paths_str"), ",")
	}

	count := 0
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			if err := fm.Delete(p); err == nil {
				count++
			}
		}
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("%d item berhasil dihapus", count),
		})
		return
	}

	http.Redirect(w, r, redirectFMPath(currentPath, activeSite), http.StatusSeeOther)
}

func (h *FileManagerHandler) FixPermissions(w http.ResponseWriter, r *http.Request) {
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	fm, activeSite, _, err := h.getFMForUser(r)
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.FormValue("path")
	if path == "" {
		path = "/"
	}

	if err := fm.FixPermissions(path); err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Gagal mereset izin: " + err.Error()})
			return
		}
		http.Error(w, "Gagal mereset izin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Izin folder (755) & file (644) dan kepemilikan user berhasil dipulihkan"})
		return
	}

	http.Redirect(w, r, redirectFMPath(path, activeSite), http.StatusSeeOther)
}

