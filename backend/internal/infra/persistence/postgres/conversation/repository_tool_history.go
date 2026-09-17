package conversation

import (
	"context"
	"time"

	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm/clause"
)

const toolHistoryEventScope = "tool_history"

// Reuse run-event storage without mixing model transcripts into UI trace payloads.
func (r *Repo) SaveMessageToolHistory(ctx context.Context, userID, conversationID, messageID uint, runID, payload string) error {
	row := models.ChatRunEvent{
		UserID: userID, ConversationID: conversationID, MessageID: messageID, RunID: runID,
		EventScope: toolHistoryEventScope, EventID: "transcript", EventType: toolHistoryEventScope,
		PayloadJSON: payload, StartedAt: time.Now(),
	}
	return translateError(r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "run_id"}, {Name: "event_scope"}, {Name: "event_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"payload_json", "updated_at"}),
	}).Create(&row).Error)
}

func (r *Repo) ListMessageToolHistories(ctx context.Context, userID uint, messageIDs []uint) (map[uint]string, error) {
	result := make(map[uint]string)
	if len(messageIDs) == 0 {
		return result, nil
	}
	var rows []models.ChatRunEvent
	err := r.db.WithContext(ctx).Model(&models.ChatRunEvent{}).
		Select("chat_run_events.message_id, chat_run_events.payload_json").
		Joins("JOIN chat_messages ON chat_messages.id = chat_run_events.message_id AND chat_messages.run_id = chat_run_events.run_id AND chat_messages.conversation_id = chat_run_events.conversation_id").
		Where("chat_run_events.user_id = ? AND chat_messages.user_id = ? AND chat_run_events.message_id IN ? AND chat_run_events.event_scope = ?", userID, userID, messageIDs, toolHistoryEventScope).
		Find(&rows).Error
	if err != nil {
		return nil, translateError(err)
	}
	for _, row := range rows {
		result[row.MessageID] = row.PayloadJSON
	}
	return result, nil
}
