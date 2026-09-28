package knowledgebase

import (
	"context"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

type fileEmbeddingSubmitter interface {
	SubmitFileEmbeddings(context.Context, uint, []string) (appembedding.TargetedSubmissionResult, error)
	ResolveFileVectorizationCapabilities(context.Context, []domainconversation.FileObject) map[string]appembedding.FileVectorizationCapability
}

func (s *Service) SetFileEmbeddingSubmitter(submitter fileEmbeddingSubmitter) {
	s.fileEmbedder = submitter
}

// Admin authorization is enforced by the route. Owner zero selects platform files only.
func (s *Service) SubmitPlatformFileEmbeddings(ctx context.Context, actorUserID uint, fileIDs []string) (appembedding.TargetedSubmissionResult, error) {
	if actorUserID == 0 {
		return appembedding.TargetedSubmissionResult{}, ErrInvalidKnowledgeBase
	}
	if s.fileEmbedder == nil {
		return appembedding.TargetedSubmissionResult{}, appembedding.ErrEmbeddingServiceNotConfigured
	}
	return s.fileEmbedder.SubmitFileEmbeddings(ctx, 0, fileIDs)
}

func (s *Service) ResolveFileVectorizationCapabilities(ctx context.Context, files []domainconversation.FileObject) map[string]appembedding.FileVectorizationCapability {
	if s.fileEmbedder == nil {
		return map[string]appembedding.FileVectorizationCapability{}
	}
	return s.fileEmbedder.ResolveFileVectorizationCapabilities(ctx, files)
}
