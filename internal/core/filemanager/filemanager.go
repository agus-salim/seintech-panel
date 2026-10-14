package filemanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileItem struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	HumanSize string    `json:"human_size"`
	Mode      string    `json:"mode"`
	ModTime   time.Time `json:"mod_time"`
	IsDir     bool      `json:"is_dir"`
	Ext       string    `json:"ext"`
	IsEditable bool     `json:"is_editable"`
}

type FileManager struct {
	sanitizer *SecuritySanitizer
}

func NewFileManager(jailRoot string) (*FileManager, error) {
	sanitizer, err := NewSecuritySanitizer(jailRoot)
	if err != nil {
		return nil, err
	}
	return &FileManager{sanitizer: sanitizer}, nil
}

// List lists all files in the relative path within jail.
func (fm *FileManager) List(subpath string) ([]FileItem, error) {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(realPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read dir: %w", err)
	}

	var items []FileItem
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		itemPath := filepath.Join(subpath, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))

		editable := !entry.IsDir() && isEditableExt(ext)

		items = append(items, FileItem{
			Name:       entry.Name(),
			Path:       filepath.ToSlash(itemPath),
			Size:       info.Size(),
			HumanSize:  formatSize(info.Size(), entry.IsDir()),
			Mode:       info.Mode().String(),
			ModTime:    info.ModTime(),
			IsDir:      entry.IsDir(),
			Ext:        ext,
			IsEditable: editable,
		})
	}

	return items, nil
}

// ReadFile reads text file content up to 10MB.
func (fm *FileManager) ReadFile(subpath string) (string, error) {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return "", err
	}

	fi, err := os.Stat(realPath)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", errors.New("cannot read a directory as a file")
	}
	if fi.Size() > 10*1024*1024 {
		return "", errors.New("file too large to edit in browser (limit 10MB)")
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// WriteFile writes text or binary content into the specified file inside jail.
func (fm *FileManager) WriteFile(subpath string, content []byte) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(realPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Atomic write
	tmpFile := realPath + ".tmp"
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, realPath)
}

// CreateDir creates a new directory inside jail.
func (fm *FileManager) CreateDir(subpath string) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}
	return os.MkdirAll(realPath, 0755)
}

// Delete removes a file or folder recursively inside jail.
func (fm *FileManager) Delete(subpath string) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}

	// Prevent deleting the root directory itself
	if realPath == fm.sanitizer.jailRoot {
		return errors.New("cannot delete jail root directory")
	}

	return os.RemoveAll(realPath)
}

// Rename renames or moves a file or folder inside jail.
func (fm *FileManager) Rename(oldSubpath, newSubpath string) error {
	oldReal, err := fm.sanitizer.ResolvePath(oldSubpath)
	if err != nil {
		return err
	}
	newReal, err := fm.sanitizer.ResolvePath(newSubpath)
	if err != nil {
		return err
	}

	if oldReal == fm.sanitizer.jailRoot {
		return errors.New("cannot rename jail root directory")
	}

	return os.Rename(oldReal, newReal)
}

// Chmod changes permissions of a file or folder.
func (fm *FileManager) Chmod(subpath string, mode os.FileMode) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}
	return os.Chmod(realPath, mode)
}

// ExtractZip extracts a zip file safely with ZipSlip protection.
func (fm *FileManager) ExtractZip(zipSubpath, destSubpath string) error {
	zipReal, err := fm.sanitizer.ResolvePath(zipSubpath)
	if err != nil {
		return err
	}
	destReal, err := fm.sanitizer.ResolvePath(destSubpath)
	if err != nil {
		return err
	}

	r, err := zip.OpenReader(zipReal)
	if err != nil {
		return fmt.Errorf("open zip failed: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// ZipSlip check
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ":") {
			return fmt.Errorf("illegal path in zip archive: %s", f.Name)
		}

		target := filepath.Join(destReal, cleanName)
		// Ensure target is still within destReal
		rel, err := filepath.Rel(destReal, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("zip path outside target destination: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func isEditableExt(ext string) bool {
	editable := map[string]bool{
		".txt": true, ".html": true, ".htm": true, ".css": true,
		".js": true, ".ts": true, ".json": true, ".xml": true,
		".php": true, ".py": true, ".sh": true, ".env": true,
		".yaml": true, ".yml": true, ".sql": true, ".md": true,
		".conf": true, ".ini": true, ".htaccess": true, ".caddy": true,
	}
	return editable[ext]
}

func formatSize(bytes int64, isDir bool) string {
	if isDir {
		return "-"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Suppress unused imports
var (
	_ = tar.NewReader
	_ = gzip.NewReader
)
