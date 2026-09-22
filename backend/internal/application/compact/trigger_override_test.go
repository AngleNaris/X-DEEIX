package compact

import (
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func TestTriggerTokensOverrideLowersEnabledRuntimeTrigger(t *testing.T) {
	cfg := config.Config{
		ContextCompactEnabled:  true,
		ContextCompactTrigger:  65536,
		ContextCompactPreserve: 1,
	}
	repo := &compactRepositoryStub{}
	svc := NewService(cfg, repo, nil)

	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "first turn"},
		{ID: 2, PublicID: "m2", ParentMessageID: ptrUint(1), Role: "assistant", Content: "first answer"},
		{ID: 3, PublicID: "m3", ParentMessageID: ptrUint(2), Role: "user", Content: "latest turn"},
		{ID: 4, PublicID: "m4", ParentMessageID: ptrUint(3), Role: "assistant", Content: "latest answer"},
	}

	// Runtime trigger disabled (0) must not be resurrected by a caller value.
	cfgZero := cfg
	cfgZero.ContextCompactTrigger = 0
	svcZero := NewService(cfgZero, repo, nil)
	if snapshot := maybeCompactForTest(t, svcZero, messages, 10); snapshot != nil {
		t.Fatalf("expected disabled runtime trigger to stay disabled, got %#v", snapshot)
	}

	// A caller value above the runtime trigger must not raise it.
	if snapshot := maybeCompactForTest(t, svc, messages, 1<<20); snapshot != nil {
		t.Fatalf("expected caller trigger above runtime value to be ignored, got %#v", snapshot)
	}
}

func maybeCompactForTest(t *testing.T, svc *Service, messages []domainconversation.Message, triggerTokens int64) *domainconversation.ContextSnapshot {
	t.Helper()
	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_trigger_override",
		Messages:            messages,
		PromptTokenEstimate: 500,
		TriggerTokens:       triggerTokens,
	})
	if err != nil {
		t.Fatalf("MaybeCompactConversation: %v", err)
	}
	return snapshot
}

func ptrUint(value uint) *uint {
	return &value
}
