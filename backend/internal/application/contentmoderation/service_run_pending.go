package contentmoderation

import (
	"context"
	"strings"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"go.uber.org/zap"
)

// SyncRunPending reapplies pending after a persistent caller ensures its run row.
// Call before enqueueing moderation checks. Unknown and ephemeral runs must not write conversation state.
func (s *Service) SyncRunPending(ctx context.Context, runID string) {
	if s == nil || s.repo == nil {
		return
	}
	runID = strings.TrimSpace(runID)
	coord := s.GetCoordinator(runID)
	if coord == nil || coord.meta.Ephemeral {
		return
	}
	if err := s.repo.UpdateRunModeration(ctx, runID, domaincm.ModerationStatePending, "", "[]"); err != nil {
		s.logWarn("content_moderation_sync_pending_failed", zap.String("run_id", runID), zap.Error(err))
	}
}
