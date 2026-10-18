package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSiteUsername(t *testing.T) {
	tests := []struct {
		domain   string
		expected string
	}{
		{"ocbt.my.id", "ocbt_my_id_usr"},
		{"www.ocbt.my.id", "ocbt_my_id_usr"},
		{"my-awesome-domain.com", "my_awesome_domain_com_usr"},
		{"123test.org", "u_123test_org_usr"},
		{"super-extra-long-domain-name-that-is-way-too-long.com", "super_extra_long_domain_name_usr"},
	}

	for _, tt := range tests {
		got := GenerateSiteUsername(tt.domain)
		if got != tt.expected {
			t.Errorf("GenerateSiteUsername(%q) = %q, expected %q", tt.domain, got, tt.expected)
		}
		if len(got) > 32 {
			t.Errorf("Generated username %q length (%d) exceeds 32 chars", got, len(got))
		}
	}
}

func TestSetupVHostDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "seintech_vhost_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	domain := "test.example.com"
	siteUser := GenerateSiteUsername(domain)

	userHome, docRoot, err := SetupVHostDirectory(tmpDir, siteUser, domain)
	if err != nil {
		t.Fatalf("SetupVHostDirectory failed: %v", err)
	}

	expectedDocRoot := filepath.Join(userHome, "data", "www", domain)
	if docRoot != expectedDocRoot {
		t.Errorf("expected docRoot %s, got %s", expectedDocRoot, docRoot)
	}

	// Verify directory exists
	info, err := os.Stat(docRoot)
	if err != nil {
		t.Fatalf("docRoot does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("docRoot is not a directory")
	}

	// Verify index.html exists
	indexFile := filepath.Join(docRoot, "index.html")
	fileInfo, err := os.Stat(indexFile)
	if err != nil {
		t.Fatalf("index.html was not created: %v", err)
	}
	if fileInfo.IsDir() {
		t.Errorf("index.html is unexpectedly a directory")
	}
}
