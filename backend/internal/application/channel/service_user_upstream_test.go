package channel

import (
	"context"
	"errors"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"testing"
)

type approvalUpstreamRepo struct {
	modelUpdateRepo
	value domainchannel.Upstream
}

func (r *approvalUpstreamRepo) GetUserUpstreamByID(context.Context, uint, uint) (*domainchannel.Upstream, error) {
	item := r.value
	return &item, nil
}
func (r *approvalUpstreamRepo) UpdateUserUpstream(_ context.Context, item *domainchannel.Upstream) error {
	r.value = *item
	return nil
}

func TestUserUpstreamCannotBypassApproval(t *testing.T) {
	repo := &approvalUpstreamRepo{value: domainchannel.Upstream{ID: 1, Status: "pending_approval"}}
	svc := NewService(config.Config{UserUpstreamEnabled: true, UserUpstreamRequireApproval: true}, repo, repo, nil, nil)
	active := "active"
	if err := svc.UpdateUserUpstream(context.Background(), 1, 1, UpdateUserUpstreamInput{Status: &active}); err != nil {
		t.Fatal(err)
	}
	if repo.value.Status != "pending_approval" {
		t.Fatal("user activated an unapproved upstream")
	}
	invalid := "approved"
	if err := svc.UpdateUserUpstream(context.Background(), 1, 1, UpdateUserUpstreamInput{Status: &invalid}); !errors.Is(err, ErrInvalidUpstreamStatus) {
		t.Fatalf("invalid status accepted: %v", err)
	}
	repo.value.Status = "active"
	headers := map[string]string{"X-Test": "new"}
	if err := svc.UpdateUserUpstream(context.Background(), 1, 1, UpdateUserUpstreamInput{Headers: &headers}); err != nil {
		t.Fatal(err)
	}
	if repo.value.Status != "pending_approval" {
		t.Fatal("credential change skipped approval")
	}
}
