package agent

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestAutoTeamWorkspaceDirUsesTenantScopedRootOnce(t *testing.T) {
	tenantID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000444")
	tenantSlug := "tenant-delta"
	teamID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000445")
	workspaceRoot := t.TempDir()
	tenantWorkspaceRoot := config.TenantWorkspace(workspaceRoot, tenantID, tenantSlug)

	loop := &Loop{
		dataDir: tenantWorkspaceRoot,
	}
	req := &RunRequest{
		UserID: "user-123",
	}
	team := &store.TeamData{
		BaseModel: store.BaseModel{ID: teamID},
	}

	got, err := loop.autoTeamWorkspaceDir(req, team)
	if err != nil {
		t.Fatalf("autoTeamWorkspaceDir returned error: %v", err)
	}

	want := filepath.Join(tenantWorkspaceRoot, "teams", teamID.String(), req.UserID)
	if got != want {
		t.Fatalf("autoTeamWorkspaceDir = %q, want %q", got, want)
	}
}
