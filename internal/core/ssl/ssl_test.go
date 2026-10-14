package ssl

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
)

func TestSSLManager_GetDomainSSLStatus_SelfSignedNotLetsEncrypt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ssl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := storage.NewRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	mgr := NewSSLManager(repo, webserver.NewRegistry(nil), filepath.Join(tempDir, "ssl"), filepath.Join(tempDir, "acme"))

	certPath := filepath.Join(tempDir, "ssl", "test.com", "fullchain.pem")
	keyPath := filepath.Join(tempDir, "ssl", "test.com", "privkey.pem")

	// Generate a self-signed certificate (which has Issuer Organization = "Seintech Hosting Panel")
	err = mgr.GenerateSelfSigned("test.com", certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateSelfSigned failed: %v", err)
	}

	domain := model.Domain{
		ID:          "dom1",
		DomainName:  "test.com",
		SSLEnabled:  true,
		SSLProvider: model.SSLProviderLetsEncrypt,
		SSLCertPath: certPath,
		SSLKeyPath:  keyPath,
	}

	status := mgr.GetDomainSSLStatus(&domain)
	if status.IsActive {
		t.Errorf("expected IsActive to be false for dummy self-signed cert claiming to be Let's Encrypt, got true")
	}
	if status.IsLetsEncrypt {
		t.Errorf("expected IsLetsEncrypt to be false, got true")
	}

	// Test SyncDomainSSLStatus
	mgr.SyncDomainSSLStatus(&domain)
	if domain.SSLEnabled {
		t.Errorf("expected SyncDomainSSLStatus to set SSLEnabled to false, got true")
	}
	if domain.SSLProvider != model.SSLProviderNone {
		t.Errorf("expected SSLProvider to be reset to none, got %s", domain.SSLProvider)
	}
}

func TestSSLManager_GetDomainSSLStatus_Inactive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ssl-test-inactive-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repo, err := storage.NewRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	mgr := NewSSLManager(repo, webserver.NewRegistry(nil), filepath.Join(tempDir, "ssl"), filepath.Join(tempDir, "acme"))

	domain := model.Domain{
		ID:          "dom2",
		DomainName:  "inactive.com",
		SSLEnabled:  false,
		SSLProvider: model.SSLProviderNone,
	}

	status := mgr.GetDomainSSLStatus(&domain)
	if status.IsActive {
		t.Errorf("expected IsActive=false for inactive domain, got true")
	}
}

func TestSSLManager_CheckCertExpiry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ssl-test-expiry-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewSSLManager(nil, nil, filepath.Join(tempDir, "ssl"), filepath.Join(tempDir, "acme"))
	certPath := filepath.Join(tempDir, "test.crt")
	keyPath := filepath.Join(tempDir, "test.key")

	err = mgr.GenerateSelfSigned("example.org", certPath, keyPath)
	if err != nil {
		t.Fatalf("GenerateSelfSigned failed: %v", err)
	}

	expiry, err := mgr.CheckCertExpiry(certPath)
	if err != nil {
		t.Fatalf("CheckCertExpiry failed: %v", err)
	}
	if expiry == nil || expiry.Before(time.Now()) {
		t.Fatalf("expected valid future expiry, got %v", expiry)
	}
}
