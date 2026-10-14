package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seintech/panel/internal/core/ssl"
	"github.com/seintech/panel/internal/core/webserver"
	"github.com/seintech/panel/internal/model"
	"github.com/seintech/panel/internal/storage"
)

func setupTestSSLHandler(t *testing.T) (*SSLHandler, *storage.Repository, string) {
	tempDir, err := os.MkdirTemp("", "ssl-handler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	repo, err := storage.NewRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	sslMgr := ssl.NewSSLManager(repo, webserver.NewRegistry(nil), filepath.Join(tempDir, "ssl"), filepath.Join(tempDir, "acme"))
	h := NewSSLHandler(repo, sslMgr)

	return h, repo, tempDir
}

func TestSSLHandler_Issue_DomainNotFound(t *testing.T) {
	h, _, tempDir := setupTestSSLHandler(t)
	defer os.RemoveAll(tempDir)

	formData := url.Values{}
	formData.Set("domain_name", "nonexistent.com")

	req := httptest.NewRequest("POST", "/ssl/issue", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["error"] != "Domain tidak ditemukan" {
		t.Errorf("expected 'Domain tidak ditemukan', got '%v'", res["error"])
	}
}

func TestSSLHandler_Issue_ParseMultipartForm(t *testing.T) {
	h, repo, tempDir := setupTestSSLHandler(t)
	defer os.RemoveAll(tempDir)

	// Save test domain
	_ = repo.UpsertDomain(model.Domain{
		ID:         "dom-123",
		DomainName: "sieko.aguss.my.id",
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("domain_id", "dom-123")
	_ = writer.WriteField("domain_name", "sieko.aguss.my.id")
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/ssl/issue", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	// The domain SHOULD be found now (not "Domain tidak ditemukan")
	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if errMsg, ok := res["error"].(string); ok && errMsg == "Domain tidak ditemukan" {
		t.Fatalf("multipart request failed: returned 'Domain tidak ditemukan' because form fields were not parsed")
	}
}

func TestSSLHandler_Issue_ParseURLEncoded(t *testing.T) {
	h, repo, tempDir := setupTestSSLHandler(t)
	defer os.RemoveAll(tempDir)

	_ = repo.UpsertDomain(model.Domain{
		ID:         "dom-456",
		DomainName: "sieko.aguss.my.id",
	})

	params := url.Values{}
	params.Set("domain_id", "dom-456")
	params.Set("domain_name", "sieko.aguss.my.id")

	req := httptest.NewRequest("POST", "/ssl/issue", strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if errMsg, ok := res["error"].(string); ok && errMsg == "Domain tidak ditemukan" {
		t.Fatalf("urlencoded request failed: returned 'Domain tidak ditemukan'")
	}
}
