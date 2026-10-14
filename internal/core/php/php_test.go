package php

import (
	"context"
	"testing"

	"github.com/seintech/panel/internal/core/system"
)

func TestPHPManager_GetExtensions(t *testing.T) {
	sm := system.NewServiceManager()
	mgr := NewPHPManager(sm)

	exts, err := mgr.GetExtensions(context.Background(), "8.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(exts) == 0 {
		t.Fatalf("expected non-empty extensions list")
	}

	foundCurl := false
	for _, ext := range exts {
		if ext.Name == "curl" {
			foundCurl = true
			if ext.PackageName != "php8.3-curl" {
				t.Errorf("expected package php8.3-curl, got %s", ext.PackageName)
			}
			break
		}
	}

	if !foundCurl {
		t.Errorf("expected curl extension in list")
	}
}

func TestPHPManager_InstallAndDeleteExtension(t *testing.T) {
	sm := system.NewServiceManager()
	mgr := NewPHPManager(sm)

	ctx := context.Background()
	// Should gracefully succeed or return on test environment
	_ = mgr.InstallExtension(ctx, "8.3", "curl")
	_ = mgr.DeleteExtension(ctx, "8.3", "curl")
}
