package conversation

import (
	"context"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"strings"
)

// UpdateConversationRun updates only an existing snapshot owned by the caller.
// Zero values are intentional updates; immutable ownership fields are never selected.
func (r *Repo) UpdateConversationRun(ctx context.Context, item *domainconversation.Run) error {
	if item == nil || strings.TrimSpace(item.RunID) == "" || item.UserID == 0 || item.ConversationID == 0 {
		return repository.ErrInvalidInput
	}
	entity := toConversationRunModel(item)
	result := r.db.WithContext(ctx).Model(&models.ConversationRun{}).
		Where("run_id = ? AND user_id = ? AND conversation_id = ?", strings.TrimSpace(item.RunID), item.UserID, item.ConversationID).
		Select("request_id", "task_type", "endpoint", "provider", "provider_protocol", "upstream_id", "upstream_model_id", "upstream_name", "requested_model_name", "platform_model_name", "routed_binding_code", "model_vendor", "model_icon", "upstream_model_name", "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "reasoning_tokens", "tool_calls_count", "first_token_latency_ms", "total_latency_ms", "status", "error_code", "error_message", "moderation_state", "moderation_event_id", "moderation_categories_json", "started_at", "ended_at").Updates(&entity)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.ConversationRun{}).Where("run_id = ?", strings.TrimSpace(item.RunID)).Count(&count).Error; err != nil {
			return translateError(err)
		}
		if count > 0 {
			return repository.ErrConflict
		}
		return repository.ErrNotFound
	}
	return nil
}
