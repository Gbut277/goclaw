package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/sessions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
)

func TestCronRunReadFilePrefersWorkspaceAndDataDirOverProcessRoot(t *testing.T) {
	tests := []struct {
		name         string
		workspaceVal string
		dataDirVal   string
		want         string
	}{
		{
			name:         "cron run prefers workspace file first",
			workspaceVal: "workspace-version",
			dataDirVal:   "data-version",
			want:         "workspace-version",
		},
		{
			name:         "cron run falls back to data dir before process root",
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

			workspaceRoot := t.TempDir()
			dataDir := t.TempDir()
			userID := "cron-user"
			relPath := filepath.Join("cli-workspaces", "job-1", "note.txt")
			effectiveWorkspace := filepath.Join(workspaceRoot, userID)

			writeAgentTestFile(t, filepath.Join(cwd, relPath), "process-root-version")
			if tt.workspaceVal != "" {
				writeAgentTestFile(t, filepath.Join(effectiveWorkspace, relPath), tt.workspaceVal)
			}
			if tt.dataDirVal != "" {
				writeAgentTestFile(t, filepath.Join(dataDir, relPath), tt.dataDirVal)
			}

			loop := &Loop{
				id:        "agent-cron",
				tenantID:  store.MasterTenantID,
				agentType: store.AgentTypeOpen,
				workspace: workspaceRoot,
				dataDir:   dataDir,
				sessions:  noopSessionStore{},
			}

			req := RunRequest{
				SessionKey: sessions.BuildCronSessionKey("agent-cron", "job-1"),
				RunID:      "cron:job-1",
				Channel:    "cron",
				UserID:     userID,
				Message:    "Read the cron note",
			}

			ctxSetup, err := loop.injectContext(context.Background(), &req)
			if err != nil {
				t.Fatalf("injectContext: %v", err)
			}
			if gotWs := tools.ToolWorkspaceFromCtx(ctxSetup.ctx); gotWs != effectiveWorkspace {
				t.Fatalf("cron workspace = %q, want %q", gotWs, effectiveWorkspace)
			}

			readTool := tools.NewReadFileTool(workspaceRoot, true)
			readTool.SetDataDir(dataDir)
			readTool.AllowPaths(filepath.Join(dataDir, "cli-workspaces"))

			got := readTool.Execute(ctxSetup.ctx, map[string]any{"path": relPath})
			if got.IsError {
				t.Fatalf("read_file returned error: %s", got.ForLLM)
			}
			if got.ForLLM != tt.want {
				t.Fatalf("read_file = %q, want %q", got.ForLLM, tt.want)
			}
		})
	}
}

func TestCronRunReadFileUsesTenantScopedDataDir(t *testing.T) {
	workspaceRoot := t.TempDir()
	dataDir := t.TempDir()
	tenantID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000333")
	tenantSlug := "tenant-bravo"
	userID := "cron-user"
	relPath := filepath.Join("cli-workspaces", "job-1", "note.txt")

	writeAgentTestFile(t, filepath.Join(dataDir, relPath), "global-version")
	writeAgentTestFile(t, filepath.Join(dataDir, "tenants", tenantSlug, relPath), "tenant-version")

	loop := &Loop{
		id:         "agent-cron",
		tenantID:   tenantID,
		tenantSlug: tenantSlug,
		agentType:  store.AgentTypeOpen,
		workspace:  workspaceRoot,
		dataDir:    dataDir,
		sessions:   noopSessionStore{},
	}

	req := RunRequest{
		SessionKey: sessions.BuildCronSessionKey("agent-cron", "job-1"),
		RunID:      "cron:job-1",
		Channel:    "cron",
		UserID:     userID,
		Message:    "Read the tenant cron note",
	}

	ctxSetup, err := loop.injectContext(context.Background(), &req)
	if err != nil {
		t.Fatalf("injectContext: %v", err)
	}
	if got := store.TenantSlugFromContext(ctxSetup.ctx); got != tenantSlug {
		t.Fatalf("tenant slug in context = %q, want %q", got, tenantSlug)
	}

	readTool := tools.NewReadFileTool(workspaceRoot, true)
	readTool.SetDataDir(dataDir)
	readTool.AllowPaths(
		filepath.Join(dataDir, "cli-workspaces"),
		filepath.Join(dataDir, "tenants"),
	)

	got := readTool.Execute(ctxSetup.ctx, map[string]any{"path": relPath})
	if got.IsError {
		t.Fatalf("read_file returned error: %s", got.ForLLM)
	}
	if got.ForLLM != "tenant-version" {
		t.Fatalf("read_file = %q, want tenant-version", got.ForLLM)
	}
}

func writeAgentTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

type noopSessionStore struct{}

func (noopSessionStore) GetOrCreate(context.Context, string) *store.SessionData {
	return &store.SessionData{}
}
func (noopSessionStore) Get(context.Context, string) *store.SessionData                 { return &store.SessionData{} }
func (noopSessionStore) AddMessage(context.Context, string, providers.Message)          {}
func (noopSessionStore) GetHistory(context.Context, string) []providers.Message         { return nil }
func (noopSessionStore) GetSummary(context.Context, string) string                      { return "" }
func (noopSessionStore) SetSummary(context.Context, string, string)                     {}
func (noopSessionStore) GetLabel(context.Context, string) string                        { return "" }
func (noopSessionStore) SetLabel(context.Context, string, string)                       {}
func (noopSessionStore) SetAgentInfo(context.Context, string, uuid.UUID, string)        {}
func (noopSessionStore) TruncateHistory(context.Context, string, int)                   {}
func (noopSessionStore) SetHistory(context.Context, string, []providers.Message)        {}
func (noopSessionStore) Reset(context.Context, string)                                  {}
func (noopSessionStore) Delete(context.Context, string) error                           { return nil }
func (noopSessionStore) Save(context.Context, string) error                             { return nil }
func (noopSessionStore) UpdateMetadata(context.Context, string, string, string, string) {}
func (noopSessionStore) AccumulateTokens(context.Context, string, int64, int64)         {}
func (noopSessionStore) IncrementCompaction(context.Context, string)                    {}
func (noopSessionStore) GetCompactionCount(context.Context, string) int                 { return 0 }
func (noopSessionStore) GetMemoryFlushCompactionCount(context.Context, string) int      { return 0 }
func (noopSessionStore) SetMemoryFlushDone(context.Context, string)                     {}
func (noopSessionStore) GetSessionMetadata(context.Context, string) map[string]string   { return nil }
func (noopSessionStore) SetSessionMetadata(context.Context, string, map[string]string)  {}
func (noopSessionStore) SetSpawnInfo(context.Context, string, string, int)              {}
func (noopSessionStore) SetContextWindow(context.Context, string, int)                  {}
func (noopSessionStore) GetContextWindow(context.Context, string) int                   { return 0 }
func (noopSessionStore) SetLastPromptTokens(context.Context, string, int, int)          {}
func (noopSessionStore) GetLastPromptTokens(context.Context, string) (int, int)         { return 0, 0 }
func (noopSessionStore) List(context.Context, string) []store.SessionInfo               { return nil }
func (noopSessionStore) ListPaged(context.Context, store.SessionListOpts) store.SessionListResult {
	return store.SessionListResult{}
}
func (noopSessionStore) ListPagedRich(context.Context, store.SessionListOpts) store.SessionListRichResult {
	return store.SessionListRichResult{}
}
func (noopSessionStore) LastUsedChannel(context.Context, string) (string, string) { return "", "" }

var _ store.SessionStore = noopSessionStore{}
