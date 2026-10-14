package filemanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileManagerListWithPermissions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "seintech_fm_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subfolder with 0755
	subDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subDir: %v", err)
	}

	// Create a text file with 0644
	textFile := filepath.Join(tmpDir, "index.php")
	if err := os.WriteFile(textFile, []byte("<?php echo 'hello';"), 0644); err != nil {
		t.Fatalf("failed to write testFile: %v", err)
	}

	fm, err := NewFileManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to init filemanager: %v", err)
	}

	items, err := fm.List("/")
	if err != nil {
		t.Fatalf("failed to list items: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	for _, item := range items {
		if item.Name == "data" {
			if !item.IsDir {
				t.Errorf("expected data to be a directory")
			}
			if item.Type != "Folder" {
				t.Errorf("expected data type to be Folder, got %s", item.Type)
			}
			if !strings.Contains(item.Access, "7") {
				t.Errorf("expected data access to contain 7, got %s", item.Access)
			}
		} else if item.Name == "index.php" {
			if item.IsDir {
				t.Errorf("expected index.php to be a file")
			}
			if item.Type != "PHP Script" {
				t.Errorf("expected index.php type to be PHP Script, got %s", item.Type)
			}
			if !strings.Contains(item.Access, "6") {
				t.Errorf("expected index.php access to contain 6, got %s", item.Access)
			}
		}
	}
}
