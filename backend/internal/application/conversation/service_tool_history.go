package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func (s *Service) persistMessageToolHistory(ctx context.Context, userID, conversationID, messageID uint, runID string, messages []llm.Message, attempts, successful []credentialWrite) error {
	_, userIndex := toolHistoryBounds(messages)
	if userIndex < 0 || userIndex+1 >= len(messages) {
		return nil
	}
	tail := messages[userIndex+1:]
	hasCalls := false
	for _, message := range tail {
		hasCalls = hasCalls || len(message.ToolCalls) > 0
	}
	if !hasCalls {
		return nil
	}
	if len(attempts) > 0 {
		tail = cloneLLMMessages(tail)
		for i := range tail {
			tail[i].ToolCalls = append([]llm.ToolCall(nil), tail[i].ToolCalls...)
			tail[i].ToolResults = append([]llm.ToolResult(nil), tail[i].ToolResults...)
		}
		applyCredentialReplacementsToLLMMessages(tail, attempts, successful)
		for i := range tail {
			for j := range tail[i].ToolResults {
				result := &tail[i].ToolResults[j]
				result.OutputJSON, _ = applyCredentialReplacementsToJSON(result.OutputJSON, attempts, successful)
				result.Error, _ = applyCredentialReplacements(result.Error, attempts, successful)
			}
		}
	}
	encoded, err := json.Marshal(tail)
	if err != nil {
		return err
	}
	// Scrub the complete serialized transcript, including tool results, before persistence.
	payload, _ := applyCredentialReplacementsToJSON(string(encoded), attempts, successful)
	return s.repo.SaveMessageToolHistory(ctx, userID, conversationID, messageID, runID, payload)
}

// Restore only branch-selected messages. The repository also binds snapshots to the message's run.
func (s *Service) replayMessageToolHistory(ctx context.Context, userID, conversationID uint, domain []model.Message, messages []llm.Message, reasoning bool) ([]llm.Message, error) {
	ids := make([]uint, 0)
	for _, message := range domain {
		if message.ID > 0 && message.ConversationID == conversationID && message.Role == "assistant" && message.RunID != "" && !stringsEqualFold(message.Status, "blocked") {
			ids = append(ids, message.ID)
		}
	}
	if len(ids) == 0 {
		return messages, nil
	}
	histories, err := s.repo.ListMessageToolHistories(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	result := make([]llm.Message, 0, len(messages))
	index := 0
	for _, message := range domain {
		if (message.Role != "user" && message.Role != "assistant" && message.Role != "system") || stringsEqualFold(message.Status, "blocked") {
			continue
		}
		if index >= len(messages) {
			return nil, fmt.Errorf("tool history message alignment failed")
		}
		if raw := histories[message.ID]; raw != "" && message.ConversationID == conversationID && message.Role == "assistant" && message.RunID != "" {
			var history []llm.Message
			if err := json.Unmarshal([]byte(raw), &history); err != nil {
				return nil, fmt.Errorf("decode tool history for message %d: %w", message.ID, err)
			}
			if !validToolHistory(history) {
				return nil, fmt.Errorf("invalid tool history for message %d", message.ID)
			}
			for i := range history {
				if !reasoning {
					history[i].ReasoningContent = ""
				}
			}
			result = append(result, history...)
		}
		result = append(result, messages[index])
		index++
	}
	return result, nil
}

// Never send orphaned or duplicate pending tool results to a provider.
func validToolHistory(messages []llm.Message) bool {
	pending := map[string]bool{}
	for _, message := range messages {
		if message.Role != "assistant" && message.Role != "tool" && message.Role != "system" {
			return false
		}
		if len(pending) > 0 && len(message.ToolResults) == 0 {
			return false
		}
		for _, call := range message.ToolCalls {
			id := strings.TrimSpace(call.ToolCallID)
			if message.Role != "assistant" || id == "" || pending[id] {
				return false
			}
			pending[id] = true
		}
		for _, result := range message.ToolResults {
			if message.Role != "tool" || !pending[result.ToolCallID] {
				return false
			}
			delete(pending, result.ToolCallID)
		}
	}
	return len(pending) == 0
}

