package conversation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type preflightCompactRepoStub struct {
	created *model.ContextSnapshot
}

func preflightMessageIDPtr(value uint) *uint {
	return &value
}

func (r *preflightCompactRepoStub) CreateContextSnapshot(ctx context.Context, item *model.ContextSnapshot) error {
	cloned := *item
	cloned.ID = 77
	r.created = &cloned
	*item = cloned
	return nil
}

func (r *preflightCompactRepoStub) GetContextSnapshotByRunID(ctx context.Context, runID string) (*model.ContextSnapshot, error) {
	return nil, repository.ErrNotFound
}

func (r *preflightCompactRepoStub) GetLatestContextSnapshot(ctx context.Context, conversationID uint) (*model.ContextSnapshot, error) {
	return nil, repository.ErrNotFound
}

func (r *preflightCompactRepoStub) UpdateConversationCompactedAt(ctx context.Context, conversationID uint, compactedAt time.Time) error {
	return nil
}

func preflightTestService(t *testing.T, cfg config.Config, repo repository.CompactRepository) *Service {
	t.Helper()
	compactSvc := appcompact.NewServiceWithRuntime(config.NewRuntime(cfg), repo, nil)
	return NewServiceWithRuntime(
		config.NewRuntime(cfg),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		compactSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestResolveContextCompactionTriggerUsesStrictestBudget(t *testing.T) {
	// 200000 - 4000 - 13000 = 183000
	caps := `{"contextWindow":200000,"maxOutputTokens":4000}`
	cfg := config.Config{
		ContextCompactTrigger: 65536,
		ContextMaxInputTokens: 31000,
	}
	if got := resolveContextCompactionTrigger(cfg, "unknown-custom-model", caps); got != 31000 {
		t.Fatalf("expected ContextMaxInputTokens to lower the trigger, got %d", got)
	}

	cfg.ContextMaxInputTokens = 0
	if got := resolveContextCompactionTrigger(cfg, "unknown-custom-model", caps); got != 65536 {
		t.Fatalf("expected runtime trigger to stay when model budget is larger, got %d", got)
	}

	// 32000 - 4000 - 13000 = 15000
	strictCaps := `{"contextWindow":32000,"maxOutputTokens":4000}`
	if got := resolveContextCompactionTrigger(cfg, "unknown-custom-model", strictCaps); got != 15000 {
		t.Fatalf("expected model capability budget to lower the trigger, got %d", got)
	}

	cfg.ContextCompactTrigger = 0
	if got := resolveContextCompactionTrigger(cfg, "unknown-custom-model", strictCaps); got != 0 {
		t.Fatalf("expected disabled runtime trigger to stay disabled, got %d", got)
	}
}

func TestMaybeCompactContextBeforePromptCreatesSnapshotForOversizedHistory(t *testing.T) {
	cfg := config.Config{
		ContextCompactEnabled:  true,
		ContextCompactTrigger:  65536,
		ContextMaxInputTokens:  1000,
		ContextCompactPreserve: 1,
	}
	repo := &preflightCompactRepoStub{}
	svc := preflightTestService(t, cfg, repo)

	large := strings.Repeat("上下文内容", 200)
	messages := []model.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: large},
		{ID: 2, PublicID: "m2", ParentMessageID: preflightMessageIDPtr(1), Role: "assistant", Content: large},
		{ID: 3, PublicID: "m3", ParentMessageID: preflightMessageIDPtr(2), Role: "user", Content: "latest"},
		{ID: 4, PublicID: "m4", ParentMessageID: preflightMessageIDPtr(3), Role: "assistant", Content: "latest answer"},
	}
	route := &channel.ResolvedRoute{UpstreamModel: "unknown-custom-model"}
	policy := contextCompactionPolicy{AdminEnabled: true, UserEnabled: true}

	snapshot := svc.maybeCompactContextBeforePrompt(
		context.Background(),
		cfg,
		policy,
		route,
		"unknown-custom-model",
		SendMessageInput{UserID: 7, ConversationID: 9, RequestID: "req_1"},
		"run_1",
		messages,
	)

	if snapshot == nil {
		t.Fatal("expected preflight compaction snapshot")
	}
	if repo.created == nil {
		t.Fatal("expected snapshot to be persisted")
	}
	if snapshot.Strategy != "token_cap" {
		t.Fatalf("expected token_cap strategy, got %q", snapshot.Strategy)
	}
	if snapshot.CoveredUntilMessageID != 2 {
		t.Fatalf("expected snapshot to cover first turn, got %#v", snapshot)
	}
}

func TestMaybeCompactContextBeforePromptSkipsWhenDisabledOrSmall(t *testing.T) {
	cfg := config.Config{
		ContextCompactEnabled:  true,
		ContextCompactTrigger:  65536,
		ContextMaxInputTokens:  100000,
		ContextCompactPreserve: 1,
	}
	repo := &preflightCompactRepoStub{}
	svc := preflightTestService(t, cfg, repo)

	messages := []model.Message{
		{ID: 1, Role: "user", Content: "hello"},
		{ID: 2, ParentMessageID: preflightMessageIDPtr(1), Role: "assistant", Content: "hi"},
		{ID: 3, ParentMessageID: preflightMessageIDPtr(2), Role: "user", Content: "latest"},
	}
	route := &channel.ResolvedRoute{UpstreamModel: "unknown-custom-model"}

	// Small history must not compact.
	if snapshot := svc.maybeCompactContextBeforePrompt(
		context.Background(), cfg, contextCompactionPolicy{AdminEnabled: true, UserEnabled: true},
		route, "unknown-custom-model", SendMessageInput{UserID: 7, ConversationID: 9}, "run_small", messages,
	); snapshot != nil {
		t.Fatalf("expected no compaction for small history, got %#v", snapshot)
	}

	// Disabled policy must never compact, even for oversized history.
	cfg.ContextMaxInputTokens = 10
	if snapshot := svc.maybeCompactContextBeforePrompt(
		context.Background(), cfg, contextCompactionPolicy{AdminEnabled: true, UserEnabled: false},
		route, "unknown-custom-model", SendMessageInput{UserID: 7, ConversationID: 9}, "run_disabled", messages,
	); snapshot != nil {
		t.Fatalf("expected disabled policy to skip compaction, got %#v", snapshot)
	}
	if repo.created != nil {
		t.Fatalf("expected no persisted snapshot, got %#v", repo.created)
	}
}

func TestEnforceFullContextAttachmentBudgetDemotesExcessToRAG(t *testing.T) {
	cfg := config.Config{
		ContextMaxInputTokens:       1000,
		FileFullContextMaxTokens:    100000,
		FileFullContextMaxBytes:     0,
		FileFullContextLimitEnabled: true,
	}
	plan := buildConversationFileContextPlan([]AttachmentInput{
		{
			FileID:        "small",
			FileName:      "small.md",
			FileCategory:  "document",
			ExtractedText: strings.Repeat("token ", 200),
			EmbedStatus:   "ready",
		},
		{
			FileID:        "large",
			FileName:      "large.md",
			FileCategory:  "document",
			ExtractedText: strings.Repeat("token ", 200),
			EmbedStatus:   "ready",
		},
	}, "auto", cfg, "unknown-custom-model", "", true)

	// Each file alone fits the per-file limit, but together they exceed the
	// aggregate full-context budget; the excess file must degrade to RAG.
	totalFullTokens := int64(0)
	for _, item := range plan.FullAttachments {
		totalFullTokens += estimateTokens(item.ExtractedText)
	}
	if totalFullTokens > fullContextAttachmentTokenBudget(cfg, "unknown-custom-model", "") {
		t.Fatalf("expected full attachments to respect aggregate budget, got %d tokens", totalFullTokens)
	}
	if len(plan.RAGAttachments) == 0 {
		t.Fatalf("expected excess retrievable file to degrade to RAG, got %#v", plan)
	}
	for _, item := range plan.RAGAttachments {
		if item.ContextMode != fileContextModeRAG {
			t.Fatalf("expected RAG context mode on demoted file, got %q", item.ContextMode)
		}
	}
}

func TestEnforceFullContextAttachmentBudgetSkipsNonRetrievableExcess(t *testing.T) {
	cfg := config.Config{
		ContextMaxInputTokens:       500,
		FileFullContextLimitEnabled: true,
	}
	plan := buildConversationFileContextPlan([]AttachmentInput{
		{
			FileID:        "first",
			FileName:      "first.md",
			FileCategory:  "document",
			ExtractedText: strings.Repeat("token ", 120),
		},
		{
			FileID:        "second",
			FileName:      "second.md",
			FileCategory:  "document",
			ExtractedText: strings.Repeat("token ", 120),
		},
	}, "auto", cfg, "unknown-custom-model", "", false)

	if len(plan.FullAttachments) != 1 || plan.FullAttachments[0].FileID != "first" {
		t.Fatalf("expected only the first file to keep full context, got %#v", plan.FullAttachments)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].FileID != "second" || plan.Skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected excess non-retrievable file to be skipped, got %#v", plan)
	}
}

func TestTrimFullContextAttachmentsKeepsTextWithinBudget(t *testing.T) {
	items := []AttachmentInput{
		{
			FileID:        "kept",
			FileName:      "kept.md",
			ExtractedText: strings.Repeat("token ", 20),
		},
		{
			FileID:        "dropped",
			FileName:      "dropped.md",
			ExtractedText: strings.Repeat("token ", 20),
		},
		{
			FileID:   "image",
			Kind:     "image",
			MimeType: "image/png",
			Current:  true,
		},
	}

	kept, skipped := trimFullContextAttachments(items, estimateTokens(strings.Repeat("token ", 20))+estimateTokens(strings.Repeat("token ", 10)))
	if len(kept) != 2 || kept[0].FileID != "kept" || kept[1].FileID != "image" {
		t.Fatalf("expected text within budget and direct image to be kept, got %#v", kept)
	}
	if len(skipped) != 1 || skipped[0].FileID != "dropped" || skipped[0].ContextMode != fileContextModeSkipped {
		t.Fatalf("expected excess text attachment to be skipped, got %#v", skipped)
	}
}
