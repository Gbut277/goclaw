package cmd

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/scheduler"
	"github.com/nextlevelbuilder/goclaw/internal/sessions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestMakeCronJobHandler_SchedulesExpectedRunRequest(t *testing.T) {
	msgBus := bus.New()
	sess := &recordingSessionStore{}
	tenantID := uuid.MustParse("0193a5b0-7000-7000-8000-000000000111")

	var (
		mu         sync.Mutex
		gotCtxTID  uuid.UUID
		gotRequest agent.RunRequest
	)
	sched := scheduler.NewScheduler(nil, scheduler.DefaultQueueConfig(), func(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error) {
		mu.Lock()
		gotCtxTID = store.TenantIDFromContext(ctx)
		gotRequest = req
		mu.Unlock()
		return &agent.RunResult{
			Content: "cron-done",
			Usage: &providers.Usage{
				PromptTokens:     12,
				CompletionTokens: 34,
			},
		}, nil
	})

	handler := makeCronJobHandler(sched, msgBus, &config.Config{}, nil, sess)
	job := &store.CronJob{
		ID:       "job-123",
		TenantID: tenantID,
		Name:     "Nightly summary",
		UserID:   "user-123",
		Payload: store.CronPayload{
			Message: "summarize today's changes",
		},
	}

	result, err := handler(job)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.Content != "cron-done" {
		t.Fatalf("result.Content = %q, want cron-done", result.Content)
	}
	if result.InputTokens != 12 || result.OutputTokens != 34 {
		t.Fatalf("result tokens = (%d,%d), want (12,34)", result.InputTokens, result.OutputTokens)
	}

	wantSessionKey := sessions.BuildCronSessionKey("default", job.ID)
	if got := sess.resetCalls(); len(got) != 1 || got[0] != wantSessionKey {
		t.Fatalf("reset calls = %#v, want [%q]", got, wantSessionKey)
	}
	if got := sess.saveCalls(); len(got) != 1 || got[0] != wantSessionKey {
		t.Fatalf("save calls = %#v, want [%q]", got, wantSessionKey)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotCtxTID != tenantID {
		t.Fatalf("tenant in scheduler ctx = %s, want %s", gotCtxTID, tenantID)
	}
	if gotRequest.SessionKey != wantSessionKey {
		t.Fatalf("SessionKey = %q, want %q", gotRequest.SessionKey, wantSessionKey)
	}
	if gotRequest.Channel != "cron" {
		t.Fatalf("Channel = %q, want cron", gotRequest.Channel)
	}
	if gotRequest.ChannelType != "" {
		t.Fatalf("ChannelType = %q, want empty", gotRequest.ChannelType)
	}
	if gotRequest.UserID != job.UserID {
		t.Fatalf("UserID = %q, want %q", gotRequest.UserID, job.UserID)
	}
	if gotRequest.RunID != "cron:"+job.ID {
		t.Fatalf("RunID = %q, want %q", gotRequest.RunID, "cron:"+job.ID)
	}
	if gotRequest.TraceName != "Cron [Nightly summary] - default" {
		t.Fatalf("TraceName = %q", gotRequest.TraceName)
	}
	if len(gotRequest.TraceTags) != 1 || gotRequest.TraceTags[0] != "cron" {
		t.Fatalf("TraceTags = %#v, want [cron]", gotRequest.TraceTags)
	}
	if !strings.Contains(gotRequest.ExtraSystemPrompt, `scheduled job "Nightly summary"`) {
		t.Fatalf("ExtraSystemPrompt missing job name: %q", gotRequest.ExtraSystemPrompt)
	}
	if !strings.Contains(gotRequest.ExtraSystemPrompt, "Delivery is not configured") {
		t.Fatalf("ExtraSystemPrompt should mention delivery not configured: %q", gotRequest.ExtraSystemPrompt)
	}
}

func TestMakeCronJobHandler_DeliversOutboundGroupMessage(t *testing.T) {
	msgBus := bus.New()
	channelMgr := channels.NewManager(msgBus)
	channelMgr.RegisterChannel("alerts", &testChannel{
		BaseChannel: channels.NewBaseChannel("alerts", msgBus, nil),
		channelType: channels.TypeTelegram,
	})

	var gotRequest agent.RunRequest
	sched := scheduler.NewScheduler(nil, scheduler.DefaultQueueConfig(), func(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error) {
		gotRequest = req
		return &agent.RunResult{Content: "group-report"}, nil
	})

	handler := makeCronJobHandler(sched, msgBus, &config.Config{}, channelMgr, &recordingSessionStore{})
	job := &store.CronJob{
		ID:       "job-group",
		TenantID: store.MasterTenantID,
		Name:     "Group report",
		AgentID:  "agent-x",
		UserID:   "group:telegram:-100",
		Payload: store.CronPayload{
			Message: "post group report",
			Deliver: true,
			Channel: "alerts",
			To:      "-100",
		},
	}

	if _, err := handler(job); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if gotRequest.Channel != "alerts" {
		t.Fatalf("Channel = %q, want alerts", gotRequest.Channel)
	}
	if gotRequest.ChannelType != channels.TypeTelegram {
		t.Fatalf("ChannelType = %q, want %q", gotRequest.ChannelType, channels.TypeTelegram)
	}
	if gotRequest.PeerKind != "group" {
		t.Fatalf("PeerKind = %q, want group", gotRequest.PeerKind)
	}
	if gotRequest.ChatID != "-100" {
		t.Fatalf("ChatID = %q, want -100", gotRequest.ChatID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out, ok := msgBus.SubscribeOutbound(ctx)
	if !ok {
		t.Fatal("expected outbound message")
	}
	if out.Channel != "alerts" || out.ChatID != "-100" || out.Content != "group-report" {
		t.Fatalf("outbound = %#v", out)
	}
	if out.Metadata["group_id"] != "-100" {
		t.Fatalf("outbound metadata = %#v, want group_id=-100", out.Metadata)
	}
}

type testChannel struct {
	*channels.BaseChannel
	channelType string
}

func (c *testChannel) Type() string                                    { return c.channelType }
func (c *testChannel) Start(context.Context) error                     { return nil }
func (c *testChannel) Stop(context.Context) error                      { return nil }
func (c *testChannel) Send(context.Context, bus.OutboundMessage) error { return nil }
func (c *testChannel) IsAllowed(string) bool                           { return true }

type recordingSessionStore struct {
	mu    sync.Mutex
	reset []string
	save  []string
}

func (s *recordingSessionStore) GetOrCreate(context.Context, string) *store.SessionData {
	return &store.SessionData{}
}
func (s *recordingSessionStore) Get(context.Context, string) *store.SessionData {
	return &store.SessionData{}
}
func (s *recordingSessionStore) AddMessage(context.Context, string, providers.Message)   {}
func (s *recordingSessionStore) GetHistory(context.Context, string) []providers.Message  { return nil }
func (s *recordingSessionStore) GetSummary(context.Context, string) string               { return "" }
func (s *recordingSessionStore) SetSummary(context.Context, string, string)              {}
func (s *recordingSessionStore) GetLabel(context.Context, string) string                 { return "" }
func (s *recordingSessionStore) SetLabel(context.Context, string, string)                {}
func (s *recordingSessionStore) SetAgentInfo(context.Context, string, uuid.UUID, string) {}
func (s *recordingSessionStore) TruncateHistory(context.Context, string, int)            {}
func (s *recordingSessionStore) SetHistory(context.Context, string, []providers.Message) {}
func (s *recordingSessionStore) Reset(_ context.Context, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset = append(s.reset, key)
}
func (s *recordingSessionStore) Delete(context.Context, string) error { return nil }
func (s *recordingSessionStore) Save(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.save = append(s.save, key)
	return nil
}
func (s *recordingSessionStore) UpdateMetadata(context.Context, string, string, string, string) {}
func (s *recordingSessionStore) AccumulateTokens(context.Context, string, int64, int64)         {}
func (s *recordingSessionStore) IncrementCompaction(context.Context, string)                    {}
func (s *recordingSessionStore) GetCompactionCount(context.Context, string) int                 { return 0 }
func (s *recordingSessionStore) GetMemoryFlushCompactionCount(context.Context, string) int      { return 0 }
func (s *recordingSessionStore) SetMemoryFlushDone(context.Context, string)                     {}
func (s *recordingSessionStore) GetSessionMetadata(context.Context, string) map[string]string {
	return nil
}
func (s *recordingSessionStore) SetSessionMetadata(context.Context, string, map[string]string) {}
func (s *recordingSessionStore) SetSpawnInfo(context.Context, string, string, int)             {}
func (s *recordingSessionStore) SetContextWindow(context.Context, string, int)                 {}
func (s *recordingSessionStore) GetContextWindow(context.Context, string) int                  { return 0 }
func (s *recordingSessionStore) SetLastPromptTokens(context.Context, string, int, int)         {}
func (s *recordingSessionStore) GetLastPromptTokens(context.Context, string) (int, int)        { return 0, 0 }
func (s *recordingSessionStore) List(context.Context, string) []store.SessionInfo              { return nil }
func (s *recordingSessionStore) ListPaged(context.Context, store.SessionListOpts) store.SessionListResult {
	return store.SessionListResult{}
}
func (s *recordingSessionStore) ListPagedRich(context.Context, store.SessionListOpts) store.SessionListRichResult {
	return store.SessionListRichResult{}
}
func (s *recordingSessionStore) LastUsedChannel(context.Context, string) (string, string) {
	return "", ""
}

func (s *recordingSessionStore) resetCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.reset...)
}

func (s *recordingSessionStore) saveCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.save...)
}

var _ store.SessionStore = (*recordingSessionStore)(nil)
var _ channels.Channel = (*testChannel)(nil)
