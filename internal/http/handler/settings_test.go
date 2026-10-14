package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/system"
	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/storage"
)

func setupTestSettingsHandler(t *testing.T) (*SettingsHandler, *storage.Repository) {
	tempDir, err := os.MkdirTemp("", "settings-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	repo, err := storage.NewRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	sm := system.NewServiceManager()
	sslMgr := ssl.NewSSLManager(repo, webserver.NewRegistry(nil), filepath.Join(tempDir, "ssl"), filepath.Join(tempDir, "acme"))
	h := NewSettingsHandler(repo, sm, sslMgr)
	return h, repo
}

func TestSettingsHandler_AddHost_And_DeleteHost(t *testing.T) {
	h, repo := setupTestSettingsHandler(t)

	// Initially verify default settings
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings error: %v", err)
	}
	if len(settings.Hosts) != 1 {
		t.Fatalf("expected 1 initial host, got %d", len(settings.Hosts))
	}
	initialHostID := settings.Hosts[0].ID

	// 1. Add secondary host
	form := url.Values{}
	form.Set("hostname", "panel2.example.com")
	form.Set("port", "8443")

	req := httptest.NewRequest(http.MethodPost, "/settings/hosts/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.AddHost(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", rr.Code)
	}

	settings, _ = repo.GetSettings()
	if len(settings.Hosts) != 2 {
		t.Fatalf("expected 2 hosts after AddHost, got %d", len(settings.Hosts))
	}
	if settings.Hosts[1].Hostname != "panel2.example.com" {
		t.Fatalf("expected hostname panel2.example.com, got %s", settings.Hosts[1].Hostname)
	}
	secondHostID := settings.Hosts[1].ID

	// 2. Prevent deleting primary host
	delPrimaryForm := url.Values{}
	delPrimaryForm.Set("host_id", initialHostID)
	reqDelPrim := httptest.NewRequest(http.MethodPost, "/settings/hosts/delete", strings.NewReader(delPrimaryForm.Encode()))
	reqDelPrim.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrDelPrim := httptest.NewRecorder()

	h.DeleteHost(rrDelPrim, reqDelPrim)
	settings, _ = repo.GetSettings()
	if len(settings.Hosts) != 2 {
		t.Fatalf("expected primary host to not be deleted, still have 2 hosts, got %d", len(settings.Hosts))
	}

	// 3. Delete secondary host
	delForm := url.Values{}
	delForm.Set("host_id", secondHostID)
	reqDel := httptest.NewRequest(http.MethodPost, "/settings/hosts/delete", strings.NewReader(delForm.Encode()))
	reqDel.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrDel := httptest.NewRecorder()

	h.DeleteHost(rrDel, reqDel)
	settings, _ = repo.GetSettings()
	if len(settings.Hosts) != 1 {
		t.Fatalf("expected 1 host remaining after deletion, got %d", len(settings.Hosts))
	}
}

func TestSettingsHandler_UpdateHost(t *testing.T) {
	h, repo := setupTestSettingsHandler(t)

	settings, _ := repo.GetSettings()
	hostID := settings.Hosts[0].ID

	form := url.Values{}
	form.Set("host_id", hostID)
	form.Set("hostname", "panel.baru.id")
	form.Set("port", "9443")

	req := httptest.NewRequest(http.MethodPost, "/settings/hosts/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.UpdateHost(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", rr.Code)
	}

	settings, _ = repo.GetSettings()
	if settings.Hosts[0].Hostname != "panel.baru.id" {
		t.Fatalf("expected updated hostname panel.baru.id, got %s", settings.Hosts[0].Hostname)
	}
	if settings.Hosts[0].Port != 9443 {
		t.Fatalf("expected updated port 9443, got %d", settings.Hosts[0].Port)
	}
	if settings.Hostname != "panel.baru.id" {
		t.Fatalf("expected primary settings.Hostname updated, got %s", settings.Hostname)
	}
}
