package filemanager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	HumanSize  string    `json:"human_size"`
	Mode       string    `json:"mode"`
	ModTime    time.Time `json:"mod_time"`
	IsDir      bool      `json:"is_dir"`
	Ext        string    `json:"ext"`
	IsEditable bool      `json:"is_editable"`
	IsArchive  bool      `json:"is_archive"`
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

// GetRelativePath converts an absolute path into relative jail path.
func (fm *FileManager) GetRelativePath(path string) string {
	return fm.sanitizer.GetRelativePath(path)
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
		archive := !entry.IsDir() && isArchiveExt(entry.Name())

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
			IsArchive:  archive,
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

// WriteStream writes data from reader into the specified file inside jail using streaming.
func (fm *FileManager) WriteStream(subpath string, src io.Reader) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(realPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Atomic write via temp file
	tmpFile := realPath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		_ = os.Remove(tmpFile)
		return err
	}
	f.Close()

	return os.Rename(tmpFile, realPath)
}

// WriteFile writes text or binary content into the specified file inside jail.
func (fm *FileManager) WriteFile(subpath string, content []byte) error {
	return fm.WriteStream(subpath, bytes.NewReader(content))
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

		if _, err := io.Copy(outFile, rc); err != nil {
			return err
		}
	}

	return nil
}

// ExtractTarGz extracts a .tar.gz, .tgz, or .tar file safely inside jail.
func (fm *FileManager) ExtractTarGz(tarGzSubpath, destSubpath string) error {
	srcReal, err := fm.sanitizer.ResolvePath(tarGzSubpath)
	if err != nil {
		return err
	}
	destReal, err := fm.sanitizer.ResolvePath(destSubpath)
	if err != nil {
		return err
	}

	f, err := os.Open(srcReal)
	if err != nil {
		return err
	}
	defer f.Close()

	var tarReader *tar.Reader
	lowerSrc := strings.ToLower(srcReal)
	if strings.HasSuffix(lowerSrc, ".tar.gz") || strings.HasSuffix(lowerSrc, ".tgz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tarReader = tar.NewReader(gzr)
	} else {
		tarReader = tar.NewReader(f)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ":") {
			return fmt.Errorf("illegal path in tar archive: %s", header.Name)
		}

		target := filepath.Join(destReal, cleanName)
		rel, err := filepath.Rel(destReal, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("tar path outside target destination: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := header.FileInfo().Mode()
			if mode == 0 {
				mode = 0644
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

// ExtractArchive auto-detects archive format and extracts safely to destSubpath.
func (fm *FileManager) ExtractArchive(archiveSubpath, destSubpath string) error {
	lower := strings.ToLower(archiveSubpath)
	if strings.HasSuffix(lower, ".zip") {
		return fm.ExtractZip(archiveSubpath, destSubpath)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar") {
		return fm.ExtractTarGz(archiveSubpath, destSubpath)
	}
	return errors.New("unsupported archive format")
}

// CompressZip compresses multiple files and directories into a destination zip inside jail.
func (fm *FileManager) CompressZip(srcSubpaths []string, destZipSubpath string) error {
	destReal, err := fm.sanitizer.ResolvePath(destZipSubpath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destReal), 0755); err != nil {
		return err
	}

	zipFile, err := os.Create(destReal)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	for _, subpath := range srcSubpaths {
		realPath, err := fm.sanitizer.ResolvePath(subpath)
		if err != nil {
			continue
		}

		fi, err := os.Stat(realPath)
		if err != nil {
			continue
		}

		baseDir := filepath.Dir(realPath)

		if fi.IsDir() {
			err = filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				relPath, err := filepath.Rel(baseDir, path)
				if err != nil {
					return err
				}
				relPath = filepath.ToSlash(relPath)

				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				header.Name = relPath
				if info.IsDir() {
					header.Name += "/"
				} else {
					header.Method = zip.Deflate
				}

				writer, err := w.CreateHeader(header)
				if err != nil {
					return err
				}

				if !info.IsDir() {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()
					_, err = io.Copy(writer, file)
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			header, err := zip.FileInfoHeader(fi)
			if err != nil {
				return err
			}
			header.Name = filepath.Base(realPath)
			header.Method = zip.Deflate

			writer, err := w.CreateHeader(header)
			if err != nil {
				return err
			}

			file, err := os.Open(realPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(writer, file)
			file.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func isArchiveExt(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar")
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
