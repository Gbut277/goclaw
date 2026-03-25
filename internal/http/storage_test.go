package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestStorageListHidesTenantRootForMaster(t *testing.T) {
	baseDir := t.TempDir()
	writeStorageTestFile(t, filepath.Join(baseDir, "master.txt"), "master")
	writeStorageTestFile(t, filepath.Join(baseDir, "tenants", "tenant-a", "secret.txt"), "tenant-secret")

	handler := NewStorageHandler(baseDir)
	req := httptest.NewRequest("GET", "/v1/storage/files", nil)
	req = req.WithContext(store.WithTenantID(context.Background(), store.MasterTenantID))
	w := httptest.NewRecorder()

	handler.handleList(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Files []struct {
			Path string `json:"path"`
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	for _, f := range resp.Files {
		if f.Path == "tenants" || strings.HasPrefix(f.Path, "tenants/") {
			t.Fatalf("master storage unexpectedly exposed tenant path %q", f.Path)
		}
	}
}

func TestStorageReadTenantRootReturnsNotFoundForMaster(t *testing.T) {
	baseDir := t.TempDir()
	writeStorageTestFile(t, filepath.Join(baseDir, "tenants", "tenant-a", "secret.txt"), "tenant-secret")

	handler := NewStorageHandler(baseDir)
	req := httptest.NewRequest("GET", "/v1/storage/files/tenants/tenant-a/secret.txt", nil)
	req = req.WithContext(store.WithTenantID(context.Background(), store.MasterTenantID))
	req.SetPathValue("path", "tenants/tenant-a/secret.txt")
	w := httptest.NewRecorder()

	handler.handleRead(w, req)
	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestStorageSizeExcludesTenantRootForMaster(t *testing.T) {
	baseDir := t.TempDir()
	writeStorageTestFile(t, filepath.Join(baseDir, "master.txt"), "12345")
	writeStorageTestFile(t, filepath.Join(baseDir, "tenants", "tenant-a", "secret.txt"), "1234567890")

	handler := NewStorageHandler(baseDir)
	req := httptest.NewRequest("GET", "/v1/storage/size", nil)
	req = req.WithContext(store.WithTenantID(context.Background(), store.MasterTenantID))
	w := httptest.NewRecorder()

	handler.handleSize(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) == 0 {
		t.Fatal("expected SSE response body")
	}

	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "data: ") {
		t.Fatalf("unexpected SSE line %q", last)
	}

	var payload struct {
		Total int64 `json:"total"`
		Files int   `json:"files"`
		Done  bool  `json:"done"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(last, "data: ")), &payload); err != nil {
		t.Fatalf("unmarshal SSE payload: %v", err)
	}
	if !payload.Done {
		t.Fatalf("expected final SSE payload, got %#v", payload)
	}
	if payload.Total != 5 {
		t.Fatalf("total = %d, want 5", payload.Total)
	}
	if payload.Files != 1 {
		t.Fatalf("files = %d, want 1", payload.Files)
	}
}

func writeStorageTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
