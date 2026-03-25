package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/tools"
)

func TestNewSubagentReadToolInheritsParentAllowPaths(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	relPath := filepath.Join("cli-workspaces", "subagent", "note.txt")
	absPath := filepath.Join(dataDir, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", absPath, err)
	}
	if err := os.WriteFile(absPath, []byte("subagent-version"), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", absPath, err)
	}

	reg := tools.NewRegistry()
	parentReadTool := tools.NewReadFileTool(workspace, true)
	parentReadTool.AllowPaths(filepath.Join(dataDir, "cli-workspaces"))
	reg.Register(parentReadTool)

	childReadTool := newSubagentReadTool(reg, workspace, dataDir, true, nil)
	got := childReadTool.Execute(context.Background(), map[string]any{"path": relPath})
	if got.IsError {
		t.Fatalf("read_file returned error: %s", got.ForLLM)
	}
	if got.ForLLM != "subagent-version" {
		t.Fatalf("read_file = %q, want subagent-version", got.ForLLM)
	}
}
