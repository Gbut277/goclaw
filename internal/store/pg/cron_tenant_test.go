package pg

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestCronListJobs_WithoutTenantFailsClosed(t *testing.T) {
	s := &PGCronStore{}

	jobs := s.ListJobs(context.Background(), true, "", "")
	if jobs == nil {
		t.Fatal("ListJobs() returned nil slice for missing tenant")
	}
	if len(jobs) != 0 {
		t.Fatalf("ListJobs() returned %d jobs, want 0", len(jobs))
	}
}

func TestCronGetRunLog_WithoutTenantFailsClosed(t *testing.T) {
	s := &PGCronStore{}

	entries, total := s.GetRunLog(context.Background(), "", 20, 0)
	if entries == nil {
		t.Fatal("GetRunLog() returned nil slice for missing tenant")
	}
	if len(entries) != 0 || total != 0 {
		t.Fatalf("GetRunLog() = (%d entries, total %d), want (0, 0)", len(entries), total)
	}
}

func TestCronScanJob_WithoutTenantReturnsError(t *testing.T) {
	s := &PGCronStore{}

	_, err := s.scanJob(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	if err == nil {
		t.Fatal("scanJob() expected error for missing tenant")
	}
}

func TestCronMutations_WithoutTenantReturnError(t *testing.T) {
	s := &PGCronStore{}
	jobID := uuid.MustParse("11111111-1111-1111-1111-111111111111").String()

	if err := s.RemoveJob(context.Background(), jobID); err == nil {
		t.Fatal("RemoveJob() expected error for missing tenant")
	}
	if err := s.EnableJob(context.Background(), jobID, true); err == nil {
		t.Fatal("EnableJob() expected error for missing tenant")
	}
	if _, err := s.UpdateJob(context.Background(), jobID, store.CronJobPatch{}); err == nil {
		t.Fatal("UpdateJob() expected error for missing tenant")
	}
}
