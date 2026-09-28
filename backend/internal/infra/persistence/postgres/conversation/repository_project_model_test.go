package conversation

import (
	"context"
	"errors"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"testing"
)

func TestProjectDefaultModelPersistUpdateAndClear(t *testing.T) {
	repo := NewRepo(openConversationRepositoryTestDB(t))
	ctx := context.Background()
	project := domainconversation.ConversationProject{UserID: 1, PublicID: "project_model", Name: "Model defaults", DefaultModel: "custom-model", Status: "active"}
	if err := repo.CreateConversationProject(ctx, &project); err != nil {
		t.Fatal(err)
	}
	assertModel := func(want string) {
		t.Helper()
		loaded, err := repo.GetConversationProjectByPublicID(ctx, 1, project.PublicID)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.DefaultModel != want {
			t.Fatalf("default model = %q, want %q", loaded.DefaultModel, want)
		}
	}
	assertModel("custom-model")
	next := "next-model"
	if _, err := repo.UpdateConversationProjectMetadataByPublicID(ctx, 2, project.PublicID, domainconversation.ConversationProjectPatch{DefaultModel: &next}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("cross-user update: %v", err)
	}
	assertModel("custom-model")
	updated, err := repo.UpdateConversationProjectMetadataByPublicID(ctx, 1, project.PublicID, domainconversation.ConversationProjectPatch{DefaultModel: &next})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DefaultModel != next {
		t.Fatalf("update response = %q", updated.DefaultModel)
	}
	assertModel(next)
	name := "Renamed"
	if _, err := repo.UpdateConversationProjectMetadataByPublicID(ctx, 1, project.PublicID, domainconversation.ConversationProjectPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	assertModel(next)
	empty := ""
	if _, err := repo.UpdateConversationProjectMetadataByPublicID(ctx, 1, project.PublicID, domainconversation.ConversationProjectPatch{DefaultModel: &empty}); err != nil {
		t.Fatal(err)
	}
	assertModel("")
}
