package conversation

import (
	"context"
	"errors"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"strings"
	"testing"
)

func TestToolCallDetailOwnershipAndCombinedByteLimit(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.ChatRunEvent{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	output := strings.Repeat("界", maxConversationToolCallDetailJSONBytes/3)
	row := models.ChatRunEvent{UserID: 1, ConversationID: 10, RunID: "run_detail", EventScope: chatRunEventScopeToolCall, EventID: "tool_detail", ToolCallID: "call_detail", OutputJSON: output, ErrorJSON: "err"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetConversationToolCallDetail(ctx, 2, row.RunID, row.ToolCallID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("other user read: %v", err)
	}
	detail, err := repo.GetConversationToolCallDetail(ctx, 1, row.RunID, row.ToolCallID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.OutputOmitted || !detail.ErrorOmitted || detail.OutputJSON != "" || detail.ErrorJSON != "" {
		t.Fatal("combined payload cap not enforced")
	}
	if detail.OutputSizeBytes != int64(len(output)) || detail.ErrorSizeBytes != 3 {
		t.Fatalf("incorrect byte sizes: %+v", detail)
	}
	if err := db.Model(&row).Updates(map[string]any{"output_json": "界", "error_json": ""}).Error; err != nil {
		t.Fatal(err)
	}
	detail, err = repo.GetConversationToolCallDetail(ctx, 1, row.RunID, row.ToolCallID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.OutputJSON != "界" || detail.OutputSizeBytes != 3 || detail.OutputOmitted || detail.ErrorOmitted {
		t.Fatalf("small payload changed: %+v", detail)
	}
}

func TestRunStatusesAreUserScoped(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.ConversationRun{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	rows := []models.ConversationRun{{RunID: "run_owned", UserID: 1, ConversationID: 10, Status: "success"}, {RunID: "run_other", UserID: 2, ConversationID: 20, Status: "running"}}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListConversationRunStatusesByRunIDs(context.Background(), 1, []string{"run_owned", "run_other"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RunID != "run_owned" || items[0].Status != "success" {
		t.Fatalf("unexpected statuses: %+v", items)
	}
	empty, err := repo.ListConversationRunStatusesByRunIDs(context.Background(), 1, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty query = %v, %v", empty, err)
	}
}
