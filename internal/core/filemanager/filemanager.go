package filemanager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FileItem struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        string    `json:"type"`
	Size        int64     `json:"size"`
	HumanSize   string    `json:"human_size"`
	Mode        string    `json:"mode"`
	Owner       string    `json:"owner"`
	Group       string    `json:"group"`
	Access      string    `json:"access"`
	ModTime     time.Time `json:"mod_time"`
	IsDir       bool      `json:"is_dir"`
	Ext         string    `json:"ext"`
	IsEditable  bool      `json:"is_editable"`
	IsArchive   bool      `json:"is_archive"`
	ArchiveType string    `json:"archive_type"`
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

		archive, archiveType := getArchiveInfo(entry.Name())
		archive = !entry.IsDir() && archive
		editable := !entry.IsDir() && isEditableExt(ext) && !archive

		owner, group, access, fileType := getFileOwnerGroup(info)

		items = append(items, FileItem{
			Name:        entry.Name(),
			Path:        filepath.ToSlash(itemPath),
			Type:        fileType,
			Size:        info.Size(),
			HumanSize:   formatSize(info.Size(), entry.IsDir()),
			Mode:        info.Mode().String(),
			Owner:       owner,
			Group:       group,
			Access:      access,
			ModTime:     info.ModTime(),
			IsDir:       entry.IsDir(),
			Ext:         ext,
			IsEditable:  editable,
			IsArchive:   archive,
			ArchiveType: archiveType,
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

	if err := os.Rename(tmpFile, realPath); err != nil {
		return err
	}

	_ = os.Chmod(realPath, 0644)
	_ = inheritParentOwnership(realPath)
	return nil
}

// WriteFile writes text or binary content into the specified file inside jail.
func (fm *FileManager) WriteFile(subpath string, content []byte) error {
	return fm.WriteStream(subpath, bytes.NewReader(content))
}

// CreateFile creates an empty regular file inside jail.
func (fm *FileManager) CreateFile(subpath string) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(realPath); err == nil {
		return errors.New("file already exists")
	}

	if err := os.MkdirAll(filepath.Dir(realPath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(realPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.Close()

	_ = inheritParentOwnership(realPath)
	return nil
}

// CreateDir creates directory inside jail.
func (fm *FileManager) CreateDir(subpath string) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(realPath, 0755); err != nil {
		return err
	}

	_ = inheritParentOwnership(realPath)
	return nil
}

// Delete deletes file or directory inside jail.
func (fm *FileManager) Delete(subpath string) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}

	// Prevent deleting jail root itself
	if realPath == fm.sanitizer.jailRoot {
		return errors.New("cannot delete root directory")
	}

	return os.RemoveAll(realPath)
}

// Rename renames file or folder inside jail.
func (fm *FileManager) Rename(oldSubpath, newSubpath string) error {
	oldReal, err := fm.sanitizer.ResolvePath(oldSubpath)
	if err != nil {
		return err
	}
	newReal, err := fm.sanitizer.ResolvePath(newSubpath)
	if err != nil {
		return err
	}

	if oldReal == fm.sanitizer.jailRoot || newReal == fm.sanitizer.jailRoot {
		return errors.New("cannot rename root directory")
	}

	return os.Rename(oldReal, newReal)
}

// Chmod changes permission mode inside jail.
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

	if err := os.MkdirAll(destReal, 0755); err != nil {
		return fmt.Errorf("gagal membuat direktori tujuan: %w", err)
	}

	// Try system unzip command if available on Linux
	if unzipPath, err := exec.LookPath("unzip"); err == nil {
		cmd := exec.Command(unzipPath, "-o", "-q", zipReal, "-d", destReal)
		if out, err := cmd.CombinedOutput(); err == nil {
			_ = inheritOwnershipRecursive(destReal)
			return nil
		} else {
			log.Printf("[FileManager] System unzip warning: %v, output: %s. Using Go unzip.", err, string(out))
		}
	}

	r, err := zip.OpenReader(zipReal)
	if err != nil {
		return fmt.Errorf("open zip failed: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		cleanName = strings.TrimPrefix(cleanName, "/")
		cleanName = strings.TrimPrefix(cleanName, "\\")
		if strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("illegal path in zip archive: %s", f.Name)
		}

		target := filepath.Join(destReal, cleanName)
		rel, err := filepath.Rel(destReal, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("zip path outside target destination: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		mode := f.Mode()
		if mode == 0 {
			mode = 0644
		}
		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}

	_ = inheritOwnershipRecursive(destReal)
	return nil
}

// ExtractTarGz extracts a .tar.gz, .tgz, .tar, .tar.bz2, or .tar.xz file safely inside jail.
func (fm *FileManager) ExtractTarGz(tarGzSubpath, destSubpath string) error {
	srcReal, err := fm.sanitizer.ResolvePath(tarGzSubpath)
	if err != nil {
		return err
	}
	destReal, err := fm.sanitizer.ResolvePath(destSubpath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destReal, 0755); err != nil {
		return fmt.Errorf("gagal membuat direktori tujuan: %w", err)
	}

	lowerSrc := strings.ToLower(srcReal)

	// Try system tar command if available (standard on Debian/Ubuntu Linux)
	if tarPath, err := exec.LookPath("tar"); err == nil {
		var cmd *exec.Cmd
		if strings.HasSuffix(lowerSrc, ".tar.gz") || strings.HasSuffix(lowerSrc, ".tgz") {
			cmd = exec.Command(tarPath, "-xzf", srcReal, "-C", destReal)
		} else if strings.HasSuffix(lowerSrc, ".tar.bz2") || strings.HasSuffix(lowerSrc, ".tbz2") {
			cmd = exec.Command(tarPath, "-xjf", srcReal, "-C", destReal)
		} else if strings.HasSuffix(lowerSrc, ".tar.xz") || strings.HasSuffix(lowerSrc, ".txz") {
			cmd = exec.Command(tarPath, "-xJf", srcReal, "-C", destReal)
		} else {
			cmd = exec.Command(tarPath, "-xf", srcReal, "-C", destReal)
		}

		out, cmdErr := cmd.CombinedOutput()
		if cmdErr == nil {
			_ = inheritOwnershipRecursive(destReal)
			return nil
		}
		log.Printf("[FileManager] System tar extraction error: %v, output: %s. Falling back to Go extractor.", cmdErr, string(out))
	}

	// Go native tar extractor fallback
	f, err := os.Open(srcReal)
	if err != nil {
		return err
	}
	defer f.Close()

	var tarReader *tar.Reader
	if strings.HasSuffix(lowerSrc, ".tar.gz") || strings.HasSuffix(lowerSrc, ".tgz") || strings.HasSuffix(lowerSrc, ".gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip reader failed: %w", err)
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
			return fmt.Errorf("tar reader error: %w", err)
		}

		cleanName := filepath.Clean(header.Name)
		cleanName = strings.TrimPrefix(cleanName, "/")
		cleanName = strings.TrimPrefix(cleanName, "\\")
		if strings.HasPrefix(cleanName, "..") {
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
		case tar.TypeSymlink:
			linkTarget := header.Linkname
			if !filepath.IsAbs(linkTarget) {
				resolvedTarget := filepath.Join(filepath.Dir(target), linkTarget)
				if relTarget, err := filepath.Rel(destReal, resolvedTarget); err == nil && !strings.HasPrefix(relTarget, "..") {
					_ = os.Remove(target)
					_ = os.Symlink(linkTarget, target)
				}
			}
		}
	}

	_ = inheritOwnershipRecursive(destReal)
	return nil
}

// ExtractArchive auto-detects archive format and extracts safely to destSubpath.
func (fm *FileManager) ExtractArchive(archiveSubpath, destSubpath string) error {
	lower := strings.ToLower(archiveSubpath)
	if strings.HasSuffix(lower, ".zip") {
		return fm.ExtractZip(archiveSubpath, destSubpath)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".gz") ||
		strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2") ||
		strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz") {
		return fm.ExtractTarGz(archiveSubpath, destSubpath)
	}
	return errors.New("format arsip tidak didukung (gunakan .zip, .tar.gz, .tgz, .tar)")
}

// CompressArchive compresses multiple files/directories to destination archive (.zip or .tar.gz).
func (fm *FileManager) CompressArchive(srcSubpaths []string, destSubpath string) error {
	lower := strings.ToLower(destSubpath)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return fm.CompressTarGz(srcSubpaths, destSubpath)
	}
	return fm.CompressZip(srcSubpaths, destSubpath)
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

	for _, sub := range srcSubpaths {
		realPath, err := fm.sanitizer.ResolvePath(sub)
		if err != nil {
			continue
		}

		fi, err := os.Lstat(realPath)
		if err != nil {
			continue
		}

		baseName := filepath.Base(realPath)
		if fi.IsDir() {
			err = filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				relPath, err := filepath.Rel(filepath.Dir(realPath), path)
				if err != nil {
					return nil
				}

				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return nil
				}
				header.Name = filepath.ToSlash(relPath)
				if info.IsDir() {
					header.Name += "/"
				} else {
					header.Method = zip.Deflate
				}

				writer, err := w.CreateHeader(header)
				if err != nil {
					return nil
				}
				if !info.IsDir() {
					file, err := os.Open(path)
					if err != nil {
						return nil
					}
					_, _ = io.Copy(writer, file)
					file.Close()
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
			header.Name = baseName
			header.Method = zip.Deflate

			writer, err := w.CreateHeader(header)
			if err != nil {
				return err
			}

			file, err := os.Open(realPath)
			if err != nil {
				return err
			}
			_, _ = io.Copy(writer, file)
			file.Close()
		}
	}

	_ = os.Chmod(destReal, 0644)
	_ = inheritParentOwnership(destReal)
	return nil
}

// CompressTarGz compresses multiple files and directories into a destination tar.gz inside jail.
func (fm *FileManager) CompressTarGz(srcSubpaths []string, destTarGzSubpath string) error {
	destReal, err := fm.sanitizer.ResolvePath(destTarGzSubpath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destReal), 0755); err != nil {
		return err
	}

	// Try system tar if on Linux
	if tarPath, err := exec.LookPath("tar"); err == nil && len(srcSubpaths) > 0 {
		var srcReals []string
		var baseDir string
		for i, sub := range srcSubpaths {
			sr, err := fm.sanitizer.ResolvePath(sub)
			if err != nil {
				continue
			}
			if i == 0 {
				baseDir = filepath.Dir(sr)
			}
			rel, err := filepath.Rel(baseDir, sr)
			if err == nil {
				srcReals = append(srcReals, rel)
			}
		}
		if len(srcReals) > 0 {
			args := append([]string{"-czf", destReal, "-C", baseDir}, srcReals...)
			cmd := exec.Command(tarPath, args...)
			if err := cmd.Run(); err == nil {
				_ = os.Chmod(destReal, 0644)
				_ = inheritParentOwnership(destReal)
				return nil
			}
		}
	}

	// Go native tar.gz compression
	tarGzFile, err := os.Create(destReal)
	if err != nil {
		return err
	}
	defer tarGzFile.Close()

	gzWriter := gzip.NewWriter(tarGzFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, sub := range srcSubpaths {
		realPath, err := fm.sanitizer.ResolvePath(sub)
		if err != nil {
			continue
		}
		fi, err := os.Lstat(realPath)
		if err != nil {
			continue
		}

		baseName := filepath.Base(realPath)
		if fi.IsDir() {
			_ = filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				relPath, err := filepath.Rel(filepath.Dir(realPath), path)
				if err != nil {
					return nil
				}
				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return nil
				}
				header.Name = filepath.ToSlash(relPath)
				if err := tarWriter.WriteHeader(header); err != nil {
					return nil
				}
				if !info.IsDir() {
					f, err := os.Open(path)
					if err == nil {
						_, _ = io.Copy(tarWriter, f)
						f.Close()
					}
				}
				return nil
			})
		} else {
			header, err := tar.FileInfoHeader(fi, "")
			if err == nil {
				header.Name = baseName
				if err := tarWriter.WriteHeader(header); err == nil {
					f, err := os.Open(realPath)
					if err == nil {
						_, _ = io.Copy(tarWriter, f)
						f.Close()
					}
				}
			}
		}
	}

	_ = os.Chmod(destReal, 0644)
	_ = inheritParentOwnership(destReal)
	return nil
}

// FixPermissions recursively restores FastPanel permissions (0755 dirs, 0644 files) and site user ownership.
func (fm *FileManager) FixPermissions(subpath string) error {
	realPath, err := fm.sanitizer.ResolvePath(subpath)
	if err != nil {
		return err
	}
	return inheritOwnershipRecursive(realPath)
}

func getArchiveInfo(name string) (bool, string) {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".tar.gz") {
		return true, "TAR.GZ"
	}
	if strings.HasSuffix(lower, ".tgz") {
		return true, "TGZ"
	}
	if strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2") {
		return true, "TAR.BZ2"
	}
	if strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz") {
		return true, "TAR.XZ"
	}
	if strings.HasSuffix(lower, ".tar") {
		return true, "TAR"
	}
	if strings.HasSuffix(lower, ".zip") {
		return true, "ZIP"
	}
	if strings.HasSuffix(lower, ".gz") {
		return true, "GZ"
	}
	if strings.HasSuffix(lower, ".bz2") {
		return true, "BZ2"
	}
	if strings.HasSuffix(lower, ".7z") {
		return true, "7Z"
	}
	if strings.HasSuffix(lower, ".rar") {
		return true, "RAR"
	}
	return false, ""
}

func isArchiveExt(name string) bool {
	archive, _ := getArchiveInfo(name)
	return archive
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
