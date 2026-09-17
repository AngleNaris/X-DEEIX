package conversation

import (
	"strings"
	"testing"

	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
)

func TestToolHistoryPersistsFullPayloadAndIsolatesRunAndOwner(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	r := NewRepo(db)
	m := models.Message{ConversationID: 2, UserID: 7, PublicID: "tool-history-message", RunID: "run-1", Role: "assistant", Content: "answer"}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	payload := `[{"Content":"` + strings.Repeat("x", (1<<20)+100) + `"}]`
	if err := r.SaveMessageToolHistory(t.Context(), 7, 2, m.ID, "run-1", payload); err != nil {
		t.Fatal(err)
	}
	got, err := r.ListMessageToolHistories(t.Context(), 7, []uint{m.ID})
	if err != nil || got[m.ID] != payload {
		t.Fatal("payload truncated", err)
	}
	got, err = r.ListMessageToolHistories(t.Context(), 8, []uint{m.ID})
	if err != nil || len(got) != 0 {
		t.Fatal("cross-user data", err)
	}
	if err := r.SaveMessageToolHistory(t.Context(), 7, 2, m.ID, "run-1", "updated"); err != nil {
		t.Fatal(err)
	}
	got, err = r.ListMessageToolHistories(t.Context(), 7, []uint{m.ID})
	if err != nil || got[m.ID] != "updated" {
		t.Fatal("snapshot update failed", err)
	}
	if err := db.Model(&m).Update("run_id", "retry-run").Error; err != nil {
		t.Fatal(err)
	}
	got, err = r.ListMessageToolHistories(t.Context(), 7, []uint{m.ID})
	if err != nil || len(got) != 0 {
		t.Fatal("replayed superseded retry", err)
	}
}
