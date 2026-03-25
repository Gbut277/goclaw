package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/sandbox"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestReadFilePrefersWorkspaceAndDataDirOverProcessRoot(t *testing.T) {
	tests := []struct {
		name         string
		workspaceVal string
		dataDirVal   string
		want         string
	}{
		{
			name:         "prefers workspace file first",
			workspaceVal: "workspace-version",
			dataDirVal:   "data-version",
			want:         "workspace-version",
		},
		{
			name:         "falls back to data dir before process root",
			workspaceVal: "",
			dataDirVal:   "data-version",
			want:         "data-version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := t.TempDir()
			oldWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("Getwd: %v", err)
			}
			if err := os.Chdir(cwd); err != nil {
				t.Fatalf("Chdir: %v", err)
			}
			t.Cleanup(func() { _ = os.Chdir(oldWD) })

			workspace := t.TempDir()
			dataDir := t.TempDir()
			relPath := filepath.Join("cli-workspaces", "session-1", "note.txt")

			writeTestFile(t, filepath.Join(cwd, relPath), "process-root-version")
			if tt.workspaceVal != "" {
				writeTestFile(t, filepath.Join(workspace, relPath), tt.workspaceVal)
			}
			if tt.dataDirVal != "" {
				writeTestFile(t, filepath.Join(dataDir, relPath), tt.dataDirVal)
			}

			tool := NewReadFileTool(workspace, true)
			tool.SetDataDir(dataDir)
			tool.AllowPaths(filepath.Join(dataDir, "cli-workspaces"))

			got := tool.Execute(context.Background(), map[string]any{"path": relPath})
			if got.IsError {
				t.Fatalf("read_file returned error: %s", got.ForLLM)
			}
			if got.ForLLM != tt.want {
				t.Fatalf("read_file = %q, want %q", got.ForLLM, tt.want)
			}
		})
	}
}

func TestReadFilePrefersTenantDataDirFromContext(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	tenantID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000222")
	tenantSlug := "tenant-alpha"
	relPath := filepath.Join("cli-workspaces", "session-1", "note.txt")

	writeTestFile(t, filepath.Join(dataDir, relPath), "global-version")
	writeTestFile(t, filepath.Join(dataDir, "tenants", tenantSlug, relPath), "tenant-version")

	tool := NewReadFileTool(workspace, true)
	tool.SetDataDir(dataDir)
	tool.AllowPaths(
		filepath.Join(dataDir, "cli-workspaces"),
		filepath.Join(dataDir, "tenants"),
	)

	ctx := store.WithTenantSlug(store.WithTenantID(context.Background(), tenantID), tenantSlug)
	got := tool.Execute(ctx, map[string]any{"path": relPath})
	if got.IsError {
		t.Fatalf("read_file returned error: %s", got.ForLLM)
	}
	if got.ForLLM != "tenant-version" {
		t.Fatalf("read_file = %q, want tenant-version", got.ForLLM)
	}
}

func TestReadFileDoesNotFallBackToGlobalDataDirForTenantContext(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	tenantID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000223")
	tenantSlug := "tenant-beta"
	relPath := filepath.Join("cli-workspaces", "session-1", "note.txt")

	writeTestFile(t, filepath.Join(dataDir, relPath), "global-version")

	tool := NewReadFileTool(workspace, true)
	tool.SetDataDir(dataDir)
	tool.AllowPaths(
		filepath.Join(dataDir, "cli-workspaces"),
		filepath.Join(dataDir, "tenants"),
	)

	ctx := store.WithTenantSlug(store.WithTenantID(context.Background(), tenantID), tenantSlug)
	got := tool.Execute(ctx, map[string]any{"path": relPath})
	if !got.IsError {
		t.Fatalf("read_file unexpectedly succeeded with %q", got.ForLLM)
	}
	if strings.Contains(got.ForLLM, "global-version") {
		t.Fatalf("read_file fell back to global data dir: %q", got.ForLLM)
	}
}

func TestReadFileTenantFallbackOnlyUsesWhitelistedSubdirs(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	tenantID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000224")
	tenantSlug := "tenant-gamma"

	writeTestFile(t, filepath.Join(dataDir, "tenants", tenantSlug, "teams", "team-a", "notes.txt"), "team-secret")
	writeTestFile(t, filepath.Join(dataDir, "tenants", tenantSlug, "media", "session-a", "notes.txt"), "media-secret")

	tool := NewReadFileTool(workspace, true)
	tool.SetDataDir(dataDir)
	tool.AllowPaths(
		filepath.Join(dataDir, "cli-workspaces"),
		filepath.Join(dataDir, "tenants"),
	)

	ctx := store.WithTenantSlug(store.WithTenantID(context.Background(), tenantID), tenantSlug)
	for _, relPath := range []string{
		filepath.Join("teams", "team-a", "notes.txt"),
		filepath.Join("media", "session-a", "notes.txt"),
	} {
		got := tool.Execute(ctx, map[string]any{"path": relPath})
		if !got.IsError {
			t.Fatalf("read_file unexpectedly succeeded for %q with %q", relPath, got.ForLLM)
		}
		if strings.Contains(got.ForLLM, "secret") {
			t.Fatalf("read_file exposed tenant data for %q: %q", relPath, got.ForLLM)
		}
	}
}

func TestReadFileRejectsExternalRootsWhenSandboxed(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	relPath := filepath.Join("cli-workspaces", "session-1", "note.txt")

	writeTestFile(t, filepath.Join(dataDir, relPath), "data-version")

	mgr := &recordingSandboxManager{}
	tool := NewSandboxedReadFileTool(workspace, true, mgr)
	tool.SetDataDir(dataDir)
	tool.AllowPaths(filepath.Join(dataDir, "cli-workspaces"))

	ctx := WithToolSandboxKey(context.Background(), "sandbox-1")
	got := tool.Execute(ctx, map[string]any{"path": relPath})
	if !got.IsError {
		t.Fatal("read_file unexpectedly succeeded for sandboxed external root")
	}
	if !strings.Contains(got.ForLLM, "outside workspace") {
		t.Fatalf("read_file error = %q, want outside workspace", got.ForLLM)
	}
	if mgr.getCalled {
		t.Fatal("sandbox manager should not be invoked for paths outside the mounted workspace")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

type recordingSandboxManager struct {
	getCalled bool
}

func (m *recordingSandboxManager) Get(context.Context, string, string, *sandbox.Config) (sandbox.Sandbox, error) {
	m.getCalled = true
	return nil, errors.New("unexpected sandbox lookup")
}

func (m *recordingSandboxManager) Release(context.Context, string) error { return nil }
func (m *recordingSandboxManager) ReleaseAll(context.Context) error      { return nil }
func (m *recordingSandboxManager) Stop()                                 {}
func (m *recordingSandboxManager) Stats() map[string]any                 { return nil }
