package system

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/seintech/panel/internal/model"
)

// GetDirSizeMB calculates the size of a directory in Megabytes (MB).
func GetDirSizeMB(dirPath string) int64 {
	if dirPath == "" {
		return 0
	}
	if _, err := os.Stat(dirPath); err != nil {
		return 0
	}

	var totalBytes int64
	_ = filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalBytes += info.Size()
		}
		return nil
	})
	return totalBytes / (1024 * 1024)
}

// GetUserRamUsageMB calculates current RSS RAM used by a Linux username in Megabytes (MB).
func GetUserRamUsageMB(ctx context.Context, username string) int64 {
	if runtime.GOOS != "linux" || username == "" {
		return 0
	}

	// ps -u <username> -o rss=
	cmd := exec.CommandContext(ctx, "ps", "-u", username, "-o", "rss=")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	var totalRSSKB int64
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if kb, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			totalRSSKB += kb
		}
	}
	return totalRSSKB / 1024
}

// CalculateUserDiskUsage calculates total disk storage in MB used by a user and their virtualhosts.
func CalculateUserDiskUsage(webRoot string, user model.User, domains []model.Domain) int64 {
	checkedDirs := make(map[string]bool)
	var totalMB int64

	// 1. User HomeDir
	if user.HomeDir != "" && !checkedDirs[user.HomeDir] {
		totalMB += GetDirSizeMB(user.HomeDir)
		checkedDirs[user.HomeDir] = true
	}

	// 2. User's Domains document roots or site users
	for _, d := range domains {
		if d.SiteUser != "" {
			siteHome := filepath.Join(webRoot, d.SiteUser)
			if !checkedDirs[siteHome] {
				totalMB += GetDirSizeMB(siteHome)
				checkedDirs[siteHome] = true
			}
		} else if d.DocumentRoot != "" && !checkedDirs[d.DocumentRoot] {
			totalMB += GetDirSizeMB(d.DocumentRoot)
			checkedDirs[d.DocumentRoot] = true
		}
	}

	return totalMB
}

// CalculateUserRamUsage calculates total RAM in MB actively used by user's processes & PHP-FPM pools.
func CalculateUserRamUsage(ctx context.Context, user model.User, domains []model.Domain) int64 {
	if runtime.GOOS != "linux" {
		return 0
	}

	checkedUsers := make(map[string]bool)
	var totalMB int64

	if user.Username != "" && !checkedUsers[user.Username] {
		totalMB += GetUserRamUsageMB(ctx, user.Username)
		checkedUsers[user.Username] = true
	}

	for _, d := range domains {
		if d.SiteUser != "" && !checkedUsers[d.SiteUser] {
			totalMB += GetUserRamUsageMB(ctx, d.SiteUser)
			checkedUsers[d.SiteUser] = true
		}
	}

	return totalMB
}
