package knowledgebase

import (
	"context"
	"errors"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"testing"
)

func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseAccessByPublicID(ctx context.Context, id string) (*domainknowledgebase.KnowledgeBase, error) {
	return s.GetKnowledgeBaseByPublicID(ctx, id)
}
func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseFileProcessingStatuses(context.Context, uint, []string) ([]domainconversation.FileObject, error) {
	return []domainconversation.FileObject{}, nil
}
func (s *knowledgeBaseRepositoryStub) GetKnowledgeBaseFileProcessingSnapshot(context.Context, uint, []string) (*repository.KnowledgeBaseFileProcessingSnapshot, error) {
	return &repository.KnowledgeBaseFileProcessingSnapshot{Files: []domainconversation.FileObject{}}, nil
}

func TestProcessingQueriesEnforceKnowledgeBaseVisibility(t *testing.T) {
	for _, tc := range []struct {
		name    string
		scope   string
		owner   uint
		enabled bool
		user    uint
		allowed bool
	}{
		{"owner", domainknowledgebase.ScopeUser, 7, true, 7, true},
		{"other user", domainknowledgebase.ScopeUser, 8, true, 7, false},
		{"disabled owned", domainknowledgebase.ScopeUser, 7, false, 7, false},
		{"enabled builtin", domainknowledgebase.ScopeBuiltin, 0, true, 7, true},
		{"disabled builtin", domainknowledgebase.ScopeBuiltin, 0, false, 7, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(&knowledgeBaseRepositoryStub{item: &domainknowledgebase.KnowledgeBase{ID: 1, Scope: tc.scope, OwnerUserID: tc.owner, Enabled: tc.enabled}})
			_, err := svc.GetVisibleFileProcessingStatuses(context.Background(), tc.user, "kb", []string{"file"})
			_, _, snapshotErr := svc.GetVisibleFileProcessingSnapshot(context.Background(), tc.user, "kb", nil)
			if tc.allowed {
				if err != nil || snapshotErr != nil {
					t.Fatalf("visible queries failed: %v / %v", err, snapshotErr)
				}
			} else if !errors.Is(err, ErrKnowledgeBaseNotFound) || !errors.Is(snapshotErr, ErrKnowledgeBaseNotFound) {
				t.Fatalf("inaccessible knowledge base exposed: %v / %v", err, snapshotErr)
			}
		})
	}
}
