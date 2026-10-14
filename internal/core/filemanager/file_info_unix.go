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
