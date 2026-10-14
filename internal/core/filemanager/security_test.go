package filemanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecuritySanitizerPaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "seintech_sec_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "sieko_aguss_my_id_usr")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subDir: %v", err)
	}

	sanitizer, err := NewSecuritySanitizer(tmpDir)
	if err != nil {
		t.Fatalf("failed to create sanitizer: %v", err)
	}

	// Test 1: GetRelativePath with "/"
	rel1 := sanitizer.GetRelativePath("/")
	if rel1 != "/" {
		t.Errorf("expected '/', got %q", rel1)
	}

	// Test 2: GetRelativePath with "/sieko_aguss_my_id_usr"
	rel2 := sanitizer.GetRelativePath("/sieko_aguss_my_id_usr")
	t.Logf("GetRelativePath('/sieko_aguss_my_id_usr') = %q", rel2)
	if rel2 != "/sieko_aguss_my_id_usr" {
		t.Errorf("expected '/sieko_aguss_my_id_usr', got %q", rel2)
	}

	// Test 3: GetRelativePath with absolute path
	rel3 := sanitizer.GetRelativePath(subDir)
	t.Logf("GetRelativePath(subDir) = %q", rel3)
	if rel3 != "/sieko_aguss_my_id_usr" {
		t.Errorf("expected '/sieko_aguss_my_id_usr', got %q", rel3)
	}

	// Test 4: ResolvePath with "/sieko_aguss_my_id_usr"
	resolved, err := sanitizer.ResolvePath("/sieko_aguss_my_id_usr")
	if err != nil {
		t.Fatalf("ResolvePath('/sieko_aguss_my_id_usr') failed: %v", err)
	}
	realSub, _ := filepath.EvalSymlinks(subDir)
	if resolved != realSub {
		t.Errorf("expected %q, got %q", realSub, resolved)
	}
}
