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

// GetRelativePath converts an absolute system path inside jail into a clean relative path for UI.
func (s *SecuritySanitizer) GetRelativePath(absPath string) string {
	rel, err := filepath.Rel(s.jailRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "/"
	}
	if rel == "." {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}
