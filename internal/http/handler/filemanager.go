package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/seintech/panel/internal/config"
	"github.com/seintech/panel/internal/core/filemanager"
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
	CurrentPath string
	ParentPath  string
	Breadcrumbs []Breadcrumb
	Files       []filemanager.FileItem
}

func (h *FileManagerHandler) getFMForUser(r *http.Request) (*filemanager.FileManager, error) {
	sess := GetSession(r)
	jailRoot := h.cfg.WebRoot

	// If regular user, jail to user's directory
	if sess.Role == "user" {
		jailRoot = filepath.Join(h.cfg.WebRoot, sess.Username)
	}

	return filemanager.NewFileManager(jailRoot)
}

func (h *FileManagerHandler) List(w http.ResponseWriter, r *http.Request) {
	fm, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, "File manager initialization error", http.StatusInternalServerError)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	items, err := fm.List(path)
	if err != nil {
		items = []filemanager.FileItem{}
	}

	settings, _ := h.repo.GetSettings()

	// Build breadcrumbs
	cleanPath := filepath.ToSlash(filepath.Clean(path))
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
		BasePageData: NewBaseData(r, "File Manager", "Web File Manager & Code Editor", "filemanager", settings.Hostname),
		CurrentPath:  cleanPath,
		ParentPath:   parent,
		Breadcrumbs:  crumbs,
		Files:        items,
	}

	_ = web.RenderTemplate(w, "filemanager.html", data)
}

func (h *FileManagerHandler) Read(w http.ResponseWriter, r *http.Request) {
	fm, err := h.getFMForUser(r)
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
	fm, err := h.getFMForUser(r)
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
	fm, err := h.getFMForUser(r)
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
	fm, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	destPath := r.FormValue("path")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed reading uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed reading file contents", http.StatusInternalServerError)
		return
	}

	target := filepath.Join(destPath, header.Filename)
	_ = fm.WriteFile(target, data)

	http.Redirect(w, r, "/filemanager?path="+destPath, http.StatusSeeOther)
}

func (h *FileManagerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	fm, err := h.getFMForUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := r.FormValue("path")
	parent := filepath.ToSlash(filepath.Dir(path))
	_ = fm.Delete(path)

	http.Redirect(w, r, "/filemanager?path="+parent, http.StatusSeeOther)
}

func (h *FileManagerHandler) CreateFile(w http.ResponseWriter, r *http.Request) {
	fm, _ := h.getFMForUser(r)
	path := r.FormValue("path")
	filename := r.FormValue("filename")
	target := filepath.Join(path, filename)
	_ = fm.WriteFile(target, []byte(""))

	http.Redirect(w, r, "/filemanager?path="+path, http.StatusSeeOther)
}

func (h *FileManagerHandler) CreateDir(w http.ResponseWriter, r *http.Request) {
	fm, _ := h.getFMForUser(r)
	path := r.FormValue("path")
	dirname := r.FormValue("dirname")
	target := filepath.Join(path, dirname)
	_ = fm.CreateDir(target)

	http.Redirect(w, r, "/filemanager?path="+path, http.StatusSeeOther)
}

func (h *FileManagerHandler) Chmod(w http.ResponseWriter, r *http.Request) {
	fm, _ := h.getFMForUser(r)
	path := r.FormValue("path")
	modeStr := r.FormValue("mode")

	val, err := strconv.ParseUint(modeStr, 8, 32)
	if err == nil {
		_ = fm.Chmod(path, os.FileMode(val))
	}

	parent := filepath.ToSlash(filepath.Dir(path))
	http.Redirect(w, r, "/filemanager?path="+parent, http.StatusSeeOther)
}

func (h *FileManagerHandler) Rename(w http.ResponseWriter, r *http.Request) {
	fm, _ := h.getFMForUser(r)
	oldPath := r.FormValue("old_path")
	newName := r.FormValue("new_name")
	parent := filepath.ToSlash(filepath.Dir(oldPath))
	newPath := filepath.Join(parent, newName)
	_ = fm.Rename(oldPath, newPath)

	http.Redirect(w, r, "/filemanager?path="+parent, http.StatusSeeOther)
}
