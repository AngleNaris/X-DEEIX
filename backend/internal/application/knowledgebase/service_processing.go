package knowledgebase

import (
	"context"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"strings"
)

func (s *Service) GetAdmin(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, ErrKnowledgeBaseNotFound
	}
	return item, nil
}

func (s *Service) GetVisibleFileProcessingStatuses(ctx context.Context, userID uint, publicID string, fileIDs []string) ([]domainconversation.FileObject, error) {
	if userID == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if !isVisibleToUser(item, userID) {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingStatuses(ctx, item.ID, fileIDs)
}

func (s *Service) GetVisibleFileProcessingSnapshot(ctx context.Context, userID uint, publicID string, fileIDs []string) (*domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	if userID == 0 {
		return nil, nil, ErrInvalidKnowledgeBase
	}
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, nil, err
	}
	if !isVisibleToUser(item, userID) {
		return nil, nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingSnapshot(ctx, item, fileIDs)
}

func (s *Service) GetAdminFileProcessingStatuses(ctx context.Context, publicID string, fileIDs []string) ([]domainconversation.FileObject, error) {
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingStatuses(ctx, item.ID, fileIDs)
}

func (s *Service) GetAdminFileProcessingSnapshot(ctx context.Context, publicID string, fileIDs []string) (*domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	item, err := s.getForAccess(ctx, publicID)
	if err != nil {
		return nil, nil, err
	}
	if item.Scope != domainknowledgebase.ScopeBuiltin {
		return nil, nil, ErrKnowledgeBaseNotFound
	}
	return s.getFileProcessingSnapshot(ctx, item, fileIDs)
}

func (s *Service) getFileProcessingSnapshot(ctx context.Context, item *domainknowledgebase.KnowledgeBase, fileIDs []string) (*domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	ids := normalizePublicIDs(fileIDs, maxFilesPerAddRequest)
	if len(fileIDs) > 0 && len(ids) == 0 {
		return nil, nil, ErrInvalidKnowledgeBase
	}
	snapshot, err := s.repo.GetKnowledgeBaseFileProcessingSnapshot(ctx, item.ID, ids)
	if err != nil {
		return nil, nil, mapRepositoryError(err)
	}
	item.FileCount = snapshot.FileCount
	item.ReadyFileCount = snapshot.ReadyFileCount
	item.ProcessingFileCount = snapshot.ProcessingFileCount
	return item, snapshot.Files, nil
}

func (s *Service) getFileProcessingStatuses(ctx context.Context, knowledgeBaseID uint, fileIDs []string) ([]domainconversation.FileObject, error) {
	ids := normalizePublicIDs(fileIDs, maxFilesPerAddRequest)
	if len(ids) == 0 {
		return nil, ErrInvalidKnowledgeBase
	}
	items, err := s.repo.GetKnowledgeBaseFileProcessingStatuses(ctx, knowledgeBaseID, ids)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return items, nil
}

func (s *Service) getForAccess(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error) {
	if strings.TrimSpace(publicID) == "" {
		return nil, ErrInvalidKnowledgeBase
	}
	item, err := s.repo.GetKnowledgeBaseAccessByPublicID(ctx, strings.TrimSpace(publicID))
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func isVisibleToUser(item *domainknowledgebase.KnowledgeBase, userID uint) bool {
	return item != nil && item.Enabled && (item.Scope == domainknowledgebase.ScopeBuiltin || (item.Scope == domainknowledgebase.ScopeUser && item.OwnerUserID == userID))
}
