package conversation

import (
	"context"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"go.uber.org/zap"
)

// resolveContextCompactionTrigger returns the strictest enabled input budget.
// A zero runtime token trigger remains an intentional opt-out for token-based
// compaction; turn-based compaction continues to be handled independently.
func resolveContextCompactionTrigger(cfg config.Config, modelName string, capabilitiesJSON string) int64 {
	trigger := int64(cfg.ContextCompactTrigger)
	if trigger <= 0 {
		return 0
	}
	if maxInput := int64(cfg.ContextMaxInputTokens); maxInput > 0 && maxInput < trigger {
		trigger = maxInput
	}
	if modelBudget := llm.CompactionThresholdFromCapabilities(modelName, capabilitiesJSON); modelBudget > 0 && modelBudget < trigger {
		trigger = modelBudget
	}
	return trigger
}

func estimateMessagesForCompaction(messages []model.Message) int64 {
	var total int64
	for _, message := range messages {
		if message.TokenUsage > 0 {
			total += message.TokenUsage
			continue
		}
		total += estimateTokens(message.Content) + 5
	}
	return total
}

func countContextUserTurns(messages []model.Message) int {
	turns := 0
	for _, message := range messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "user") {
			turns++
		}
	}
	return turns
}

// maybeCompactContextBeforePrompt creates a snapshot before building the final
// upstream request. The existing post-billing compaction remains responsible
// for asynchronous rolling maintenance after successful responses.
func (s *Service) maybeCompactContextBeforePrompt(
	ctx context.Context,
	cfg config.Config,
	policy contextCompactionPolicy,
	route *channel.ResolvedRoute,
	conversationModel string,
	input SendMessageInput,
	runID string,
	messages []model.Message,
) *model.ContextSnapshot {
	if s == nil || s.compactSvc == nil || route == nil || !policy.EffectiveEnabled() || len(messages) == 0 {
		return nil
	}

	triggerTokens := resolveContextCompactionTrigger(cfg, route.UpstreamModel, route.ModelCapabilitiesJSON)
	promptTokenEstimate := estimateMessagesForCompaction(messages)
	exceedsTurnCap := cfg.ContextMaxTurns > 0 && countContextUserTurns(messages) > cfg.ContextMaxTurns
	exceedsTokenCap := triggerTokens > 0 && promptTokenEstimate > triggerTokens
	if !exceedsTurnCap && !exceedsTokenCap {
		return nil
	}

	compactModelName := s.resolveTextTaskModel(
		ctx,
		cfg.CompactTaskModel,
		conversationModel,
		input.UserID,
		input.ConversationID,
		input.RequestID,
	)
	snapshot, err := s.compactSvc.MaybeCompactConversation(ctx, appcompact.MaybeCompactConversationInput{
		ConversationID:      input.ConversationID,
		UserID:              input.UserID,
		RunID:               runID,
		Messages:            messages,
		PromptTokenEstimate: promptTokenEstimate,
		TriggerTokens:       triggerTokens,
		PlatformModelName:   compactModelName,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("preflight_context_compaction_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", input.ConversationID),
				zap.Int64("trigger_tokens", triggerTokens),
				zap.Int64("prompt_token_estimate", promptTokenEstimate),
				zap.Error(err),
			)
		}
		return nil
	}
	if snapshot != nil {
		s.invalidateSnapshotCache(input.ConversationID)
	}
	return snapshot
}
