package embedding

import (
	"context"
	"fmt"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"strings"
)

const (
	WorkerConcurrency = 4
	MaxTargetedFiles  = 100
)

const (
	SkipReasonNotFound     = "not_found"
	SkipReasonNotReady     = "not_ready"
	SkipReasonUnsupported  = "unsupported"
	SkipReasonAlreadyReady = "already_ready"
	SkipReasonProcessing   = "processing"
	SkipReasonQueueBusy    = "queue_busy"
	SkipReasonSubmitFailed = "submit_failed"
	ReasonOutdatedIndex    = "outdated_index"
)

type TargetedFileSkip struct {
	FileID string
	Reason string
}

type TargetedSubmissionResult struct {
	SubmittedFileIDs []string
	Skipped          []TargetedFileSkip
}

type TargetedJob struct {
	FileID             string
	UserID             uint
	EmbeddingSignature string
	EmbeddingHost      string
}

type TargetedSubmissionPlan struct {
	Jobs    []TargetedJob
	Skipped []TargetedFileSkip
}

type FileVectorizationCapability struct {
	CanVectorize bool
	Reason       string
}

// PlanFiles 校验当前用户指定文件并生成向量化任务计划。
// 任务认领与投递由 processing 应用服务逐项完成，避免批量预认领后因中途失败遗留 processing 状态。
func (s *Service) PlanFiles(ctx context.Context, userID uint, fileIDs []string) (TargetedSubmissionPlan, error) {
	plan := TargetedSubmissionPlan{
		Jobs:    []TargetedJob{},
		Skipped: []TargetedFileSkip{},
	}
	normalizedIDs := normalizeTargetedFileIDs(fileIDs)
	if len(normalizedIDs) > MaxTargetedFiles {
		return plan, ErrTooManyTargetedFiles
	}
	if len(normalizedIDs) == 0 {
		return plan, nil
	}

	cfg := s.snapshot()
	available, reason, err := s.indexingAvailable(ctx, cfg)
	if !available {
		return plan, embeddingAvailabilityError(reason, err)
	}

	files, err := s.repo.GetActiveFileObjectsByIDs(ctx, userID, normalizedIDs)
	if err != nil {
		return plan, err
	}
	filesByID := make(map[string]domainconversation.FileObject, len(files))
	for i := range files {
		filesByID[files[i].FileID] = files[i]
	}

	embeddingSignature := configuredModelSignature(cfg)
	embeddingHost := strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/")
	for _, fileID := range normalizedIDs {
		fileObj, found := filesByID[fileID]
		if !found {
			plan.Skipped = append(plan.Skipped, TargetedFileSkip{FileID: fileID, Reason: SkipReasonNotFound})
			continue
		}
		if reason := fileVectorizationSkipReason(cfg, fileObj, embeddingSignature); reason != "" {
			plan.Skipped = append(plan.Skipped, TargetedFileSkip{FileID: fileID, Reason: reason})
			continue
		}

		plan.Jobs = append(plan.Jobs, TargetedJob{
			FileID:             fileID,
			UserID:             userID,
			EmbeddingSignature: embeddingSignature,
			EmbeddingHost:      embeddingHost,
		})
	}
	return plan, nil
}

// QueueTargetedJob 原子登记单个已规划任务，防止并发提交产生重复队列消息。
// 真正的 processing 状态由 worker 领取消息后再设置。
func (s *Service) QueueTargetedJob(ctx context.Context, job TargetedJob) (bool, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(job.FileID) == "" || strings.TrimSpace(job.EmbeddingSignature) == "" {
		return false, nil
	}
	return s.repo.QueueFileEmbedding(ctx, job.UserID, job.FileID, job.EmbeddingSignature)
}

// FailTargetedJob 将投递失败的已领取任务释放为可重试状态。
func (s *Service) FailTargetedJob(ctx context.Context, job TargetedJob, cause error) error {
	return s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "failed", cause)
}

// RequeueTargetedJob 将等待重试的任务恢复为排队状态。
func (s *Service) RequeueTargetedJob(ctx context.Context, job TargetedJob, cause error) error {
	return s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "queued", cause)
}

// ProcessTargetedJob 执行从可恢复队列中领取的显式向量化任务。
func (s *Service) ProcessTargetedJob(ctx context.Context, job TargetedJob) error {
	if s == nil || s.repo == nil || strings.TrimSpace(job.FileID) == "" {
		return nil
	}
	cfg := s.snapshot()
	if configuredModelSignature(cfg) != strings.TrimSpace(job.EmbeddingSignature) ||
		strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/") != strings.TrimRight(strings.TrimSpace(job.EmbeddingHost), "/") {
		_ = s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "stale", errEmbeddingConfigurationChanged)
		return nil
	}
	available, reason, err := s.indexingAvailable(ctx, cfg)
	if !available {
		switch reason {
		case "embedding_disabled", "embedding_model_missing", "embedding_host_missing":
			_ = s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "stale", errEmbeddingConfigurationChanged)
			return nil
		default:
			return embeddingAvailabilityError(reason, err)
		}
	}
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, job.UserID, job.FileID)
	if err != nil || fileObj == nil {
		return err
	}
	if fileObj.EmbedSignature != job.EmbeddingSignature || strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus)) != "processing" {
		claimed, claimErr := s.repo.ClaimFileEmbedding(ctx, job.UserID, job.FileID, job.EmbeddingSignature)
		if claimErr != nil || !claimed {
			return claimErr
		}
	}
	return s.ProcessFile(ctx, *fileObj)
}

// ResolveFileVectorizationCapabilities 返回前端展示所需的后端事实状态。
func (s *Service) ResolveFileVectorizationCapabilities(
	ctx context.Context,
	files []domainconversation.FileObject,
) map[string]FileVectorizationCapability {
	capabilities := make(map[string]FileVectorizationCapability, len(files))
	cfg := s.snapshot()
	signature := configuredModelSignature(cfg)
	available, reason, _ := s.indexingAvailable(ctx, cfg)
	if !available {
		for i := range files {
			capabilityReason := reason
			if fileVectorIndexOutdated(files[i], signature) {
				capabilityReason = ReasonOutdatedIndex
			}
			capabilities[files[i].FileID] = FileVectorizationCapability{Reason: capabilityReason}
		}
		return capabilities
	}
	for i := range files {
		skipReason := fileVectorizationSkipReason(cfg, files[i], signature)
		reason := skipReason
		if reason == "" && fileVectorIndexOutdated(files[i], signature) {
			reason = ReasonOutdatedIndex
		}
		capabilities[files[i].FileID] = FileVectorizationCapability{
			CanVectorize: skipReason == "",
			Reason:       reason,
		}
	}
	return capabilities
}

func fileVectorizationSkipReason(cfg config.Config, fileObj domainconversation.FileObject, embeddingSignature string) string {
	if fileObj.EmbedSignature == embeddingSignature {
		switch strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus)) {
		case "ready":
			return SkipReasonAlreadyReady
		case "queued", "processing":
			return SkipReasonProcessing
		}
	}
	if !fileObj.ProcessingReady {
		return SkipReasonNotReady
	}
	if !canEmbedFile(cfg, fileObj) {
		return SkipReasonUnsupported
	}
	return ""
}

func fileVectorIndexOutdated(fileObj domainconversation.FileObject, embeddingSignature string) bool {
	status := strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus))
	return status == "stale" || (status == "ready" && strings.TrimSpace(embeddingSignature) != "" && fileObj.EmbedSignature != embeddingSignature)
}

func normalizeTargetedFileIDs(fileIDs []string) []string {
	normalized := make([]string, 0, len(fileIDs))
	seen := make(map[string]struct{}, len(fileIDs))
	for _, value := range fileIDs {
		fileID := strings.TrimSpace(value)
		if fileID == "" {
			continue
		}
		if _, exists := seen[fileID]; exists {
			continue
		}
		seen[fileID] = struct{}{}
		normalized = append(normalized, fileID)
	}
	return normalized
}

func embeddingAvailabilityError(reason string, cause error) error {
	if cause != nil {
		return fmt.Errorf("%w: %w", ErrEmbeddingServiceUnavailable, cause)
	}
	if reason == "embedding_disabled" || reason == "embedding_model_missing" || reason == "embedding_host_missing" {
		return ErrEmbeddingServiceNotConfigured
	}
	return ErrEmbeddingServiceUnavailable
}
