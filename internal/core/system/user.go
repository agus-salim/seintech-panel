package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// GenerateSiteUsername converts a domain name into a FastPanel-style Linux username.
// Example: "ocbt.my.id" -> "ocbt_my_id_usr"
func GenerateSiteUsername(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))

	// Remove common prefixes like www.
	d = strings.TrimPrefix(d, "www.")

	// Replace dots, dashes, and other non-alphanumeric chars with underscore
	re := regexp.MustCompile(`[^a-z0-9]`)
	cleaned := re.ReplaceAllString(d, "_")

	// Collapse consecutive underscores
	multiUnderscore := regexp.MustCompile(`_+`)
	cleaned = multiUnderscore.ReplaceAllString(cleaned, "_")
	cleaned = strings.Trim(cleaned, "_")

	if cleaned == "" {
		cleaned = "site"
	}

	// Linux usernames should not start with a digit
	if cleaned[0] >= '0' && cleaned[0] <= '9' {
		cleaned = "u_" + cleaned
	}

	// Linux usernames have a 32-character limit (UT_NAMESIZE).
	// With suffix "_usr" (4 chars), the base name should not exceed 28 chars.
	if len(cleaned) > 28 {
		cleaned = cleaned[:28]
		cleaned = strings.TrimRight(cleaned, "_")
	}

	return cleaned + "_usr"
}

// CreateSiteUser creates a dedicated Linux system group & user for the virtualhost (FastPanel style).
// On Windows / non-Linux dev environments, it returns nil without error.
func CreateSiteUser(ctx context.Context, username, groupname, homeDir string) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if groupname == "" {
		groupname = username
	}

	// 1. Create dedicated group if it doesn't exist
	if err := exec.CommandContext(ctx, "getent", "group", groupname).Run(); err != nil {
		// Group does not exist, create it
		cmd := exec.CommandContext(ctx, "groupadd", groupname)
		if out, err := cmd.CombinedOutput(); err != nil {
			// If already exists, ignore error
			if !strings.Contains(string(out), "already exists") {
				return fmt.Errorf("groupadd %s failed: %s (%w)", groupname, string(out), err)
			}
		}
	}

	// 2. Create dedicated user if it doesn't exist
	if err := exec.CommandContext(ctx, "id", "-u", username).Run(); err != nil {
		// Determine nologin shell path
		shell := "/usr/sbin/nologin"
		if _, err := os.Stat(shell); err != nil {
			if _, err := os.Stat("/sbin/nologin"); err == nil {
				shell = "/sbin/nologin"
			} else {
				shell = "/bin/false"
			}
		}

		// useradd -m -d <homeDir> -g <groupname> -s <shell> <username>
		args := []string{"-m"}
		if homeDir != "" {
			args = append(args, "-d", homeDir)
		}
		args = append(args, "-g", groupname, "-s", shell, username)

		cmd := exec.CommandContext(ctx, "useradd", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			if !strings.Contains(string(out), "already exists") {
				return fmt.Errorf("useradd %s failed: %s (%w)", username, string(out), err)
			}
		}
	} else if homeDir != "" {
		// If user already exists, ensure home directory is configured
		_ = exec.CommandContext(ctx, "usermod", "-d", homeDir, username).Run()
	}

	// 3. Add web server users (www-data, nginx, apache) to the site user's group
	// so the web servers can read web files while respecting 755/644 permissions.
	for _, webUser := range []string{"www-data", "nginx", "apache"} {
		if err := exec.CommandContext(ctx, "id", "-u", webUser).Run(); err == nil {
			_ = exec.CommandContext(ctx, "usermod", "-a", "-G", groupname, webUser).Run()
		}
	}

	return nil
}

// DeleteSiteUser deletes the system user and group when a virtualhost is permanently removed.
func DeleteSiteUser(ctx context.Context, username, groupname string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if username != "" && username != "root" && username != "www-data" {
		_ = exec.CommandContext(ctx, "userdel", username).Run()
	}
	if groupname != "" && groupname != "root" && groupname != "www-data" {
		_ = exec.CommandContext(ctx, "groupdel", groupname).Run()
	}
	return nil
}

// SetVHostPermissions sets standard FastPanel permissions:
// - All folders are set to 0755
// - All files are set to 0644
// - Owner and group are set to username:groupname (on Linux)
func SetVHostPermissions(rootPath, username, groupname string) error {
	if rootPath == "" {
		return fmt.Errorf("rootPath cannot be empty")
	}

	// Verify target exists
	if _, err := os.Stat(rootPath); err != nil {
		return err
	}

	var uid, gid int = -1, -1
	if runtime.GOOS == "linux" && username != "" {
		if u, err := user.Lookup(username); err == nil {
			if parsedUid, err := strconv.Atoi(u.Uid); err == nil {
				uid = parsedUid
			}
		}
		if groupname != "" {
			if g, err := user.LookupGroup(groupname); err == nil {
				if parsedGid, err := strconv.Atoi(g.Gid); err == nil {
					gid = parsedGid
				}
			}
		} else if uid != -1 {
			gid = uid
		}
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			_ = os.Chmod(path, 0755)
		} else {
			_ = os.Chmod(path, 0644)
		}

		if runtime.GOOS == "linux" && uid != -1 && gid != -1 {
			_ = os.Chown(path, uid, gid)
		}

		return nil
	})

	return err
}

// SetupVHostDirectory creates the FastPanel directory structure for a site user:
// - userHome: <baseWebRoot>/<siteUser> (0755)
// - dataDir:  <baseWebRoot>/<siteUser>/data (0755)
// - wwwDir:   <baseWebRoot>/<siteUser>/data/www (0755)
// - docRoot:  <baseWebRoot>/<siteUser>/data/www/<domainName> (0755)
// - default index.php file (0644)
//
// And sets ownership to siteUser:siteUser with 755 dirs and 644 files.
func SetupVHostDirectory(baseWebRoot, siteUser, domainName string) (string, string, error) {
	if baseWebRoot == "" {
		baseWebRoot = "/var/www"
	}
	if siteUser == "" {
		siteUser = GenerateSiteUsername(domainName)
	}

	userHome := filepath.Join(baseWebRoot, siteUser)
	dataDir := filepath.Join(userHome, "data")
	wwwDir := filepath.Join(dataDir, "www")
	docRoot := filepath.Join(wwwDir, domainName)
	logsDir := filepath.Join(dataDir, "logs")

	// Create directories with 0755
	for _, dir := range []string{userHome, dataDir, wwwDir, docRoot, logsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		_ = os.Chmod(dir, 0755)
	}

	// Create default index.php if empty
	indexFile := filepath.Join(docRoot, "index.php")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		defaultContent := fmt.Sprintf(`<?php
$rewriteActive = isset($_GET['rewrite']) && $_GET['rewrite'] === 'active';
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to %s</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; padding: 1rem; }
        .card { background: #1e293b; border: 1px solid #334155; border-radius: 16px; padding: 2.5rem; max-width: 520px; text-align: center; box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5); }
        h1 { color: #38bdf8; margin-top: 0; font-size: 1.75rem; }
        p { color: #94a3b8; font-size: 0.95rem; line-height: 1.6; }
        .badge { display: inline-block; background: #0284c7; color: white; padding: 0.35rem 0.85rem; border-radius: 9999px; font-size: 0.8rem; font-weight: 600; margin-top: 1rem; }
        .status-box { margin-top: 1.25rem; padding: 0.75rem 1rem; border-radius: 12px; font-size: 0.85rem; text-align: left; }
        .status-success { background: rgba(16, 185, 129, 0.15); border: 1px solid #10b981; color: #6ee7b7; }
        .status-info { background: rgba(56, 189, 248, 0.1); border: 1px solid #0284c7; color: #bae6fd; }
        code { background: #0f172a; padding: 0.2rem 0.4rem; border-radius: 6px; font-family: monospace; font-size: 0.85rem; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Welcome to %s</h1>
        <p>Virtual host ini aktif dan berjalan lancar menggunakan user <strong>%s</strong>.</p>
        
        <?php if ($rewriteActive): ?>
        <div class="status-box status-success">
            <strong>✅ .htaccess Berfungsi Sempurna!</strong><br>
            URL rewrite <code>/test-rewrite</code> berhasil diproses oleh Apache <code>mod_rewrite</code>.
        </div>
        <?php else: ?>
        <div class="status-box status-info">
            <strong>💡 Uji Coba .htaccess:</strong><br>
            Buka <a href="/test-rewrite" style="color:#38bdf8;text-decoration:underline;">/test-rewrite</a> untuk menguji kerja aturan rewrite pada file <code>.htaccess</code>.
        </div>
        <?php endif; ?>

        <div style="margin-top: 1.5rem;">
            <span class="badge">PHP <?php echo PHP_VERSION; ?> &bull; Powered by Seintech Panel</span>
        </div>
    </div>
</body>
</html>
`, domainName, domainName, siteUser)

		if err := os.WriteFile(indexFile, []byte(defaultContent), 0644); err != nil {
			return "", "", fmt.Errorf("failed to write default index.php: %w", err)
		}
		_ = os.Chmod(indexFile, 0644)
	}

	// Create default .htaccess if not present
	EnsureDomainHtaccess(docRoot, siteUser, siteUser)

	// Recursively apply 0755 for dirs and 0644 for files, with ownership
	_ = SetVHostPermissions(userHome, siteUser, siteUser)

	return userHome, docRoot, nil
}

// EnsureDomainHtaccess creates a standard sample .htaccess in docRoot if missing.
func EnsureDomainHtaccess(docRoot, siteUser, siteGroup string) {
	if docRoot == "" {
		return
	}
	_ = os.MkdirAll(docRoot, 0755)
	htaccessFile := filepath.Join(docRoot, ".htaccess")
	if _, err := os.Stat(htaccessFile); os.IsNotExist(err) {
		htaccessContent := `# ==============================================================================
# Seintech Panel - Apache VirtualHost Configuration (.htaccess)
# ==============================================================================
<IfModule mod_rewrite.c>
    RewriteEngine On
    RewriteBase /

    # Uji coba rewrite: buka /test-rewrite untuk memastikan .htaccess aktif
    RewriteRule ^test-rewrite$ index.php?rewrite=active [L,QSA]

    # Standard Front Controller (WordPress, Laravel, CodeIgniter, etc.)
    RewriteCond %{REQUEST_FILENAME} !-f
    RewriteCond %{REQUEST_FILENAME} !-d
    RewriteRule . /index.php [L]
</IfModule>

# Security Headers & Directory Options
<IfModule mod_headers.c>
    Header set X-Content-Type-Options "nosniff"
    Header set X-Frame-Options "SAMEORIGIN"
</IfModule>

Options -Indexes +FollowSymLinks
`
		_ = os.WriteFile(htaccessFile, []byte(htaccessContent), 0644)
		_ = os.Chmod(htaccessFile, 0644)
		if siteUser != "" {
			_ = SetVHostPermissions(docRoot, siteUser, siteGroup)
		}
	}
}
