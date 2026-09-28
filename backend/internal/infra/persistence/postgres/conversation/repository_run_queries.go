package conversation

import (
	"context"
	"fmt"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
)

const maxConversationToolCallDetailJSONBytes = 8 * 1024 * 1024

func conversationEventJSONSizeExpression(db *gorm.DB, column string) string {
	switch column {
	case "payload_json", "input_json", "output_json", "error_json":
	default:
		panic("unsupported conversation event JSON column")
	}
	if db != nil && db.Dialector != nil {
		switch db.Dialector.Name() {
		case "postgres":
			return fmt.Sprintf("OCTET_LENGTH(%s)", column)
		case "sqlite":
			return fmt.Sprintf("LENGTH(CAST(%s AS BLOB))", column)
		}
	}
	return fmt.Sprintf("LENGTH(%s)", column)
}

func conversationToolCallDetailSelectColumns(db *gorm.DB) []string {
	outputSize := conversationEventJSONSizeExpression(db, "output_json")
	errorSize := conversationEventJSONSizeExpression(db, "error_json")
	totalSize := "(" + outputSize + " + " + errorSize + ")"
	return []string{
		"run_id", "tool_call_id", "tool_name", "status",
		outputSize + " AS output_size_bytes",
		fmt.Sprintf("CASE WHEN %s > 0 AND %s > %d THEN TRUE ELSE FALSE END AS output_omitted", outputSize, totalSize, maxConversationToolCallDetailJSONBytes),
		fmt.Sprintf("CASE WHEN %s <= %d THEN output_json ELSE '' END AS output_json", totalSize, maxConversationToolCallDetailJSONBytes),
		errorSize + " AS error_size_bytes",
		fmt.Sprintf("CASE WHEN %s > 0 AND %s > %d THEN TRUE ELSE FALSE END AS error_omitted", errorSize, totalSize, maxConversationToolCallDetailJSONBytes),
		fmt.Sprintf("CASE WHEN %s <= %d THEN error_json ELSE '' END AS error_json", totalSize, maxConversationToolCallDetailJSONBytes),
	}
}

func (r *Repo) GetConversationToolCallDetail(ctx context.Context, userID uint, runID, toolCallID string) (*domainconversation.ToolCallDetail, error) {
	var item models.ChatRunEvent
	if err := r.db.WithContext(ctx).Select(conversationToolCallDetailSelectColumns(r.db)).Where("user_id = ? AND run_id = ? AND event_scope = ? AND tool_call_id = ?", userID, runID, chatRunEventScopeToolCall, toolCallID).Take(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return &domainconversation.ToolCallDetail{RunID: item.RunID, ToolCallID: item.ToolCallID, ToolName: item.ToolName, Status: item.Status, OutputJSON: item.OutputJSON, OutputSizeBytes: item.OutputSizeBytes, OutputOmitted: item.OutputOmitted, ErrorJSON: item.ErrorJSON, ErrorSizeBytes: item.ErrorSizeBytes, ErrorOmitted: item.ErrorOmitted}, nil
}

func (r *Repo) ListConversationRunStatusesByRunIDs(ctx context.Context, userID uint, runIDs []string) ([]domainconversation.RunStatus, error) {
	items := make([]domainconversation.RunStatus, 0, len(runIDs))
	if len(runIDs) == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).Model(&models.ConversationRun{}).Select("run_id", "status").Where("user_id = ? AND run_id IN ?", userID, runIDs).Order("id ASC").Scan(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return items, nil
}
