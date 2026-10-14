package filemanager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal   = errors.New("access denied: path traversal attempt detected")
	ErrSymlinkBreakout = errors.New("access denied: symlink resolves outside allowed directory")
	ErrInvalidPath     = errors.New("invalid file path")
)

// SecuritySanitizer provides chroot-like path containment for user operations.
type SecuritySanitizer struct {
	jailRoot string
}

func NewSecuritySanitizer(jailRoot string) (*SecuritySanitizer, error) {
	cleanRoot := filepath.Clean(jailRoot)
	// Create directory if not exists
	if err := os.MkdirAll(cleanRoot, 0755); err != nil {
		return nil, fmt.Errorf("cannot create jail root: %w", err)
	}

	// Resolve any existing symlinks in jailRoot
	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		realRoot = cleanRoot
	}

	return &SecuritySanitizer{
		jailRoot: realRoot,
	}, nil
}

// ResolvePath safely joins the jail root with the requested subpath and guarantees
// that the resulting path is strictly confined within the jailRoot.
func (s *SecuritySanitizer) ResolvePath(subpath string) (string, error) {
	// Normalize slashes
	cleanSub := filepath.Clean(filepath.ToSlash(subpath))
	if cleanSub == "." || cleanSub == "/" || cleanSub == "" {
		return s.jailRoot, nil
	}

	cleanJail := filepath.Clean(filepath.ToSlash(s.jailRoot))

	// If subpath is an absolute path or starts with jailRoot (e.g. /var/www/domain.com/public_html),
	// strip jailRoot from the beginning so it resolves cleanly within jailRoot!
	if strings.HasPrefix(cleanSub, cleanJail) {
		cleanSub = strings.TrimPrefix(cleanSub, cleanJail)
		cleanSub = filepath.Clean(filepath.ToSlash(cleanSub))
		if cleanSub == "." || cleanSub == "/" || cleanSub == "" {
			return s.jailRoot, nil
		}
	}

	if rel, err := filepath.Rel(s.jailRoot, cleanSub); err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
		cleanSub = rel
	}

	// Disallow absolute drive letters or rooted paths in subpath
	cleanSub = strings.TrimPrefix(cleanSub, "/")
	cleanSub = strings.TrimPrefix(cleanSub, "\\")

	target := filepath.Join(s.jailRoot, cleanSub)

	// Step 1: Lexical boundary check using filepath.Rel
	rel, err := filepath.Rel(s.jailRoot, target)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", ErrPathTraversal
	}

	// Step 2: Symlink breakout check if file/folder exists
	if fi, err := os.Lstat(target); err == nil {
		realTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate symlink: %w", err)
		}

		relReal, err := filepath.Rel(s.jailRoot, realTarget)
		if err != nil || strings.HasPrefix(relReal, "..") || relReal == ".." {
			return "", ErrSymlinkBreakout
		}

		_ = fi
		return realTarget, nil
	} else if os.IsNotExist(err) {
		// Target doesn't exist yet (e.g. creating new file).
		// Check that its parent directory resolves safely within jailRoot.
		parentDir := filepath.Dir(target)
		if _, pErr := os.Stat(parentDir); pErr == nil {
			realParent, err := filepath.EvalSymlinks(parentDir)
			if err != nil {
				return "", fmt.Errorf("failed to evaluate parent symlink: %w", err)
			}
			relParent, err := filepath.Rel(s.jailRoot, realParent)
			if err != nil || strings.HasPrefix(relParent, "..") || relParent == ".." {
				return "", ErrSymlinkBreakout
			}
		}
	}

	return target, nil
}

// GetRelativePath converts an absolute system path or jail-relative subpath into a clean relative path for UI.
func (s *SecuritySanitizer) GetRelativePath(inputPath string) string {
	cleanInput := filepath.Clean(filepath.ToSlash(inputPath))
	if cleanInput == "." || cleanInput == "/" || cleanInput == "" {
		return "/"
	}

	// Try checking if inputPath is an absolute path inside jailRoot
	if evalPath, err := filepath.EvalSymlinks(cleanInput); err == nil {
		if rel, err := filepath.Rel(s.jailRoot, evalPath); err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			if rel == "." {
				return "/"
			}
			return "/" + filepath.ToSlash(rel)
		}
	}

	if rel, err := filepath.Rel(s.jailRoot, cleanInput); err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
		if rel == "." {
			return "/"
		}
		return "/" + filepath.ToSlash(rel)
	}

	cleanJail := filepath.Clean(filepath.ToSlash(s.jailRoot))
	if strings.HasPrefix(cleanInput, cleanJail) {
		rel, err := filepath.Rel(cleanJail, cleanInput)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			if rel == "." {
				return "/"
			}
			return "/" + filepath.ToSlash(rel)
		}
	}

	// Otherwise, inputPath is already a jail-relative subpath (e.g. "/sieko_aguss_my_id_usr" or "sieko_aguss_my_id_usr")
	trimmed := strings.TrimLeft(cleanInput, "/\\")
	if trimmed == "" || trimmed == "." {
		return "/"
	}

	target := filepath.Join(s.jailRoot, trimmed)
	rel, err := filepath.Rel(s.jailRoot, target)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "/"
	}

	if rel == "." {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}
