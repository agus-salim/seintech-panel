//go:build !windows

package filemanager

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func getFileOwnerGroup(info os.FileInfo) (owner string, group string, access string, fileType string) {
	perm := info.Mode().Perm()
	oct := fmt.Sprintf("%03o", perm)
	if len(oct) == 3 {
		access = string(oct[0]) + " " + string(oct[1]) + " " + string(oct[2])
	} else {
		access = oct
	}

	owner = "-"
	group = "-"

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uidStr := strconv.FormatUint(uint64(stat.Uid), 10)
		gidStr := strconv.FormatUint(uint64(stat.Gid), 10)

		owner = uidStr
		if u, err := user.LookupId(uidStr); err == nil && u.Username != "" {
			owner = u.Username
		}

		group = gidStr
		if g, err := user.LookupGroupId(gidStr); err == nil && g.Name != "" {
			group = g.Name
		}
	}

	fileType = determineFileType(info)
	return owner, group, access, fileType
}

// inheritParentOwnership sets the newly created file or directory's UID and GID
// to match the parent directory (or the site user under /var/www/<site_user>).
func inheritParentOwnership(targetPath string) error {
	dir := filepath.Dir(targetPath)
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	uid := int(stat.Uid)
	gid := int(stat.Gid)

	// If parent dir is owned by root (UID 0), find the site user from path hierarchy
	if uid == 0 {
		if siteUid, siteGid, ok := findSiteUserInPath(targetPath); ok {
			uid = siteUid
			gid = siteGid
		}
	}

	if uid != 0 || gid != 0 {
		_ = os.Chown(targetPath, uid, gid)
	}
	return nil
}

// inheritOwnershipRecursive walks rootPath and applies parent/site user ownership
// as well as FastPanel permissions (0755 for dirs, 0644 for files).
func inheritOwnershipRecursive(rootPath string) error {
	dir := filepath.Dir(rootPath)
	info, err := os.Stat(dir)
	if err != nil {
		info, err = os.Stat(rootPath)
		if err != nil {
			return err
		}
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	uid := int(stat.Uid)
	gid := int(stat.Gid)

	if uid == 0 {
		if siteUid, siteGid, ok := findSiteUserInPath(rootPath); ok {
			uid = siteUid
			gid = siteGid
		}
	}

	if uid == 0 && gid == 0 {
		return nil
	}

	return filepath.Walk(rootPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			_ = os.Chmod(path, 0755)
		} else {
			_ = os.Chmod(path, 0644)
		}
		_ = os.Chown(path, uid, gid)
		return nil
	})
}

// findSiteUserInPath extracts the site username from the path hierarchy.
// Supports /var/www/<site_user>/... and usernames ending with _usr.
func findSiteUserInPath(path string) (int, int, bool) {
	clean := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(clean, "/")

	// Check path segments
	for i, part := range parts {
		if part == "www" && i+1 < len(parts) {
			candidate := parts[i+1]
			if candidate != "" && candidate != "html" && candidate != "var" {
				if u, err := user.Lookup(candidate); err == nil {
					uid, err1 := strconv.Atoi(u.Uid)
					gid, err2 := strconv.Atoi(u.Gid)
					if err1 == nil && err2 == nil {
						return uid, gid, true
					}
				}
			}
		}
	}

	// Check any segment with FastPanel _usr convention
	for _, part := range parts {
		if strings.HasSuffix(part, "_usr") {
			if u, err := user.Lookup(part); err == nil {
				uid, err1 := strconv.Atoi(u.Uid)
				gid, err2 := strconv.Atoi(u.Gid)
				if err1 == nil && err2 == nil {
					return uid, gid, true
				}
			}
		}
	}

	return 0, 0, false
}

func determineFileType(info os.FileInfo) string {
	if info.IsDir() {
		return "Folder"
	}
	name := strings.ToLower(info.Name())
	ext := strings.ToLower(filepath.Ext(name))

	if ext == ".php" || ext == ".phtml" {
		return "PHP Script"
	}
	if ext == ".html" || ext == ".htm" {
		return "HTML"
	}
	if ext == ".txt" || ext == ".md" || ext == ".htaccess" || ext == ".conf" ||
		ext == ".ini" || ext == ".json" || ext == ".yaml" || ext == ".yml" ||
		ext == ".css" || ext == ".js" || ext == ".ts" || ext == ".sql" ||
		ext == ".env" || ext == ".xml" || ext == ".csv" || ext == ".log" {
		return "Text"
	}
	if ext == ".zip" || ext == ".gz" || ext == ".tar" || ext == ".tgz" || ext == ".bz2" || ext == ".7z" || ext == ".rar" {
		return "Archive"
	}
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".svg" || ext == ".ico" {
		return "Image"
	}
	if ext == ".pdf" {
		return "PDF"
	}
	if ext == ".mp3" || ext == ".wav" || ext == ".ogg" {
		return "Audio"
	}
	if ext == ".mp4" || ext == ".mkv" || ext == ".webm" {
		return "Video"
	}
	return "File"
}
