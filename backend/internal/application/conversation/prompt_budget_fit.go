package conversation

import (
	"context"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"go.uber.org/zap"
	"strings"
)

type promptBudgetFit struct {
	Budget         int64
	TokensBefore   int64
	TokensAfter    int64
	MessagesBefore int
	MessagesAfter  int
	Trimmed        bool
	Exceeded       bool
}

func (s *Service) logPromptBudgetFit(ctx context.Context, modelName string, fit promptBudgetFit) {
	if s == nil || s.logger == nil {
		return
	}
	fields := []zap.Field{
		zap.String("trace_id", traceid.FromContext(ctx)),
		zap.String("model", strings.TrimSpace(modelName)),
		zap.Int64("effective_budget", fit.Budget),
	}
	if fit.Trimmed {
		s.logger.Info("context_prompt_budget_trimmed", append(fields,
			zap.Int64("tokens_before", fit.TokensBefore),
			zap.Int64("tokens_after", fit.TokensAfter),
			zap.Int("messages_before", fit.MessagesBefore),
			zap.Int("messages_after", fit.MessagesAfter),
		)...)
	}
	if fit.Exceeded {
		s.logger.Warn("context_prompt_required_content_exceeds_budget", append(fields,
			zap.Int64("estimated_tokens", fit.TokensAfter),
		)...)
	}
}

// fitGenerateInputToModelBudget 是初始请求的唯一硬预算入口。它在消息、原生
// instructions 与工具定义全部确定后删除最早的完整历史轮次，保留系统前缀和
// 当前用户轮次。必需内容本身超限时不会破坏当前输入，而是显式返回 Exceeded
// 供调用方记录诊断信息。
func fitGenerateInputToModelBudget(
	input llm.GenerateInput,
	modelName string,
	capabilitiesJSON string,
	fallbackContextWindow int,
	enabled bool,
) (llm.GenerateInput, promptBudgetFit) {
	result := promptBudgetFit{
		MessagesBefore: len(input.Messages),
		MessagesAfter:  len(input.Messages),
		TokensBefore:   estimateGenerateInputTokens(input),
	}
	result.TokensAfter = result.TokensBefore
	if !enabled {
		return input, result
	}

	result.Budget = int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback(
		modelName,
		capabilitiesJSON,
		fallbackContextWindow,
	))
	if result.TokensBefore <= result.Budget {
		return input, result
	}

	trimmedMessages, trimmed := trimOldestPromptHistory(input.Messages, result.TokensBefore, result.Budget)
	if trimmed {
		input.Messages = trimmedMessages
		result.Trimmed = true
		result.MessagesAfter = len(trimmedMessages)
		result.TokensAfter = estimateGenerateInputTokens(input)
	}
	result.Exceeded = result.TokensAfter > result.Budget
	return input, result
}

// trimOldestPromptHistory 删除最早的完整对话轮次，并始终保留前导系统消息与
// 当前用户轮次。传入的 totalTokens 已包含 tools/instructions 等固定开销。
func trimOldestPromptHistory(messages []llm.Message, totalTokens int64, budget int64) ([]llm.Message, bool) {
	if totalTokens <= budget || len(messages) == 0 {
		return messages, false
	}
	systemEnd, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex <= systemEnd {
		return messages, false
	}

	remainingTokens := totalTokens
	for cutFrom := systemEnd; cutFrom < currentUserIndex; cutFrom++ {
		remainingTokens -= estimateMessageTokens(messages[cutFrom])
		nextIndex := cutFrom + 1
		if nextIndex < currentUserIndex && messages[nextIndex].Role != "user" {
			continue
		}
		if remainingTokens <= budget || nextIndex == currentUserIndex {
			trimmed := make([]llm.Message, 0, systemEnd+len(messages)-nextIndex)
			trimmed = append(trimmed, messages[:systemEnd]...)
			trimmed = append(trimmed, messages[nextIndex:]...)
			return trimmed, true
		}
	}
	return messages, false
}
