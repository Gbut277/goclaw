package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type mockCronStore struct {
	listJobs                []store.CronJob
	runLog                  []store.CronRunLogEntry
	total                   int
	status                  map[string]any
	lastListIncludeDisabled bool
	lastListAgentID         string
	lastListUserID          string
}

func (m *mockCronStore) AddJob(context.Context, string, store.CronSchedule, string, bool, string, string, string, string) (*store.CronJob, error) {
	return nil, nil
}

func (m *mockCronStore) GetJob(context.Context, string) (*store.CronJob, bool) { return nil, false }

func (m *mockCronStore) ListJobs(_ context.Context, includeDisabled bool, agentID, userID string) []store.CronJob {
	m.lastListIncludeDisabled = includeDisabled
	m.lastListAgentID = agentID
	m.lastListUserID = userID
	return m.listJobs
}

func (m *mockCronStore) RemoveJob(context.Context, string) error { return nil }

func (m *mockCronStore) UpdateJob(context.Context, string, store.CronJobPatch) (*store.CronJob, error) {
	return nil, nil
}

func (m *mockCronStore) EnableJob(context.Context, string, bool) error { return nil }

func (m *mockCronStore) GetRunLog(context.Context, string, int, int) ([]store.CronRunLogEntry, int) {
	return m.runLog, m.total
}

func (m *mockCronStore) Status() map[string]any {
	if m.status == nil {
		return map[string]any{}
	}
	return m.status
}

func (m *mockCronStore) Start() error { return nil }

func (m *mockCronStore) Stop() {}

func (m *mockCronStore) SetOnJob(func(*store.CronJob) (*store.CronJobResult, error)) {}

func (m *mockCronStore) SetOnEvent(func(store.CronEvent)) {}

func (m *mockCronStore) RunJob(context.Context, string, bool) (bool, string, error) {
	return false, "", nil
}

func (m *mockCronStore) SetDefaultTimezone(string) {}

func (m *mockCronStore) GetDueJobs(time.Time) []store.CronJob { return nil }

func TestCronToolHandleList_UsesEmptyArrayWhenStoreReturnsNil(t *testing.T) {
	tool := NewCronTool(&mockCronStore{})

	result := tool.handleList(context.Background(), map[string]any{"includeDisabled": true})
	if result.IsError {
		t.Fatalf("handleList returned error: %s", result.ForLLM)
	}

	var payload struct {
		Jobs  []store.CronJob `json:"jobs"`
		Count int             `json:"count"`
	}
	if err := json.Unmarshal([]byte(result.ForLLM), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if payload.Jobs == nil {
		t.Fatal("expected jobs to marshal as an empty array, got null")
	}
	if payload.Count != 0 {
		t.Fatalf("count = %d, want 0", payload.Count)
	}
}

func TestCronToolHandleList_UsesTenantWideScope(t *testing.T) {
	mockStore := &mockCronStore{
		listJobs: []store.CronJob{{ID: "job-1"}},
	}
	tool := NewCronTool(mockStore)
	ctx := store.WithUserID(context.Background(), "system")
	ctx = store.WithAgentID(ctx, uuid.MustParse("11111111-1111-1111-1111-111111111111"))

	result := tool.handleList(ctx, map[string]any{"includeDisabled": true})
	if result.IsError {
		t.Fatalf("handleList returned error: %s", result.ForLLM)
	}
	if !mockStore.lastListIncludeDisabled {
		t.Fatal("expected includeDisabled = true")
	}
	if mockStore.lastListAgentID != "" || mockStore.lastListUserID != "" {
		t.Fatalf("expected tenant-wide list args, got agentID=%q userID=%q", mockStore.lastListAgentID, mockStore.lastListUserID)
	}
}

func TestCronToolHandleRuns_UsesEmptyArrayWhenStoreReturnsNil(t *testing.T) {
	tool := NewCronTool(&mockCronStore{})

	result := tool.handleRuns(context.Background(), map[string]any{"limit": 20}, "", "")
	if result.IsError {
		t.Fatalf("handleRuns returned error: %s", result.ForLLM)
	}

	var payload struct {
		Entries []store.CronRunLogEntry `json:"entries"`
		Count   int                     `json:"count"`
		Total   int                     `json:"total"`
	}
	if err := json.Unmarshal([]byte(result.ForLLM), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if payload.Entries == nil {
		t.Fatal("expected entries to marshal as an empty array, got null")
	}
	if payload.Count != 0 {
		t.Fatalf("count = %d, want 0", payload.Count)
	}
	if payload.Total != 0 {
		t.Fatalf("total = %d, want 0", payload.Total)
	}
}

func TestCronToolHandleStatus_UsesTenantVisibleJobCount(t *testing.T) {
	mockStore := &mockCronStore{
		listJobs: []store.CronJob{
			{ID: "job-1"},
			{ID: "job-2"},
		},
		status: map[string]any{
			"enabled": true,
			"jobs":    99, // global scheduler count
		},
	}
	tool := NewCronTool(mockStore)
	ctx := store.WithUserID(context.Background(), "system")
	ctx = store.WithAgentID(ctx, uuid.MustParse("11111111-1111-1111-1111-111111111111"))

	result := tool.handleStatus(ctx)
	if result.IsError {
		t.Fatalf("handleStatus returned error: %s", result.ForLLM)
	}

	var payload struct {
		Enabled bool `json:"enabled"`
		Jobs    int  `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(result.ForLLM), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !payload.Enabled {
		t.Fatal("enabled = false, want true")
	}
	if payload.Jobs != 2 {
		t.Fatalf("jobs = %d, want 2 for tenant-visible list", payload.Jobs)
	}
	if mockStore.lastListAgentID != "" || mockStore.lastListUserID != "" {
		t.Fatalf("expected tenant-wide status count args, got agentID=%q userID=%q", mockStore.lastListAgentID, mockStore.lastListUserID)
	}
}
