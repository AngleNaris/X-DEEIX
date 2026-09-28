package conversation

import (
	"context"
	"errors"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"testing"
)

func TestRunSnapshotUpsertCannotTransferOwnership(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.ConversationRun{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	run := domainconversation.Run{RunID: "run_ownership", UserID: 1, ConversationID: 10, Status: "running", InputTokens: 42}
	if err := repo.UpsertConversationRun(ctx, &run); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []struct{ user, conversation uint }{{2, 10}, {1, 20}, {2, 20}} {
		other := run
		other.UserID, other.ConversationID = owner.user, owner.conversation
		if err := repo.UpsertConversationRun(ctx, &other); !errors.Is(err, repository.ErrConflict) {
			t.Fatalf("ownership change returned %v", err)
		}
	}
	run.InputTokens = 0
	run.Status = "success"
	if err := repo.UpsertConversationRun(ctx, &run); err != nil {
		t.Fatal(err)
	}
	var stored models.ConversationRun
	if err := db.Where("run_id = ?", run.RunID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.UserID != 1 || stored.ConversationID != 10 || stored.InputTokens != 0 || stored.Status != "success" {
		t.Fatalf("unexpected snapshot: %+v", stored)
	}
	missing := run
	missing.RunID = "run_missing"
	if err := repo.UpdateConversationRun(ctx, &missing); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("missing update returned %v", err)
	}
	if err := repo.UpsertConversationRun(ctx, nil); !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("nil upsert returned %v", err)
	}
}
