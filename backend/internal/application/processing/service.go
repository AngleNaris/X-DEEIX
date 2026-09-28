package processing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// DefaultExtractorVersion 是当前文件处理流水线版本标识。
	DefaultExtractorVersion  = "file-pipeline-v1"
	fileProcessingMaxRetries = 3
	defaultProcessingPreview = 280
	defaultExtractTimeout    = 60 * time.Second
	fixedEmbeddingTimeout        = 5 * time.Minute
	failurePersistTimeout        = 5 * time.Second
	processingQueueFailureMessage = "文件处理失败，请稍后重试。"
	fileProcessingLeaseRenew      = 15 * time.Second
	fallbackProcessingConcurrency = 4
	fallbackProcessingTimeout     = 30 * time.Minute
)

var (
	// ErrFileProcessingFailed 表示文件处理失败。
	ErrFileProcessingFailed           = errors.New("file processing failed")
	errExtractionServiceNotConfigured = errors.New("extraction service not configured")
	ErrFileNotFound                   = errors.New("file not found")
	errFileProcessingClaimLost        = errors.New("file processing claim lost")
)

// FileProcessingStatusDTO 文件处理状态响应数据。
type FileProcessingStatusDTO struct {
	FileID              string
	DetectedMIME        string
	FileCategory        string
	ProcessingStatus    string
	ProcessingReady     bool
	ExtractStatus       string
	EmbedStatus         string
	PreviewText         string
	OCRUsed             bool
	RAGReady            bool
	RAGReason           string
	ErrorCode           string
	ErrorMessage        string
	ExtractChars        int
	ExtractPages        int
	ChunkCount          int
	EmbedError          string
	CanVectorize        bool
	VectorizationReason string
	StartedAt           *time.Time
	CompletedAt         *time.Time
	UpdatedAt           time.Time
}

// ReadyFileResult 表示等待文件处理完成后的可消费结果。
type ReadyFileResult struct {
	File          domainconversation.FileObject
	DetectedMIME  string
	FileCategory  string
	PageCount     int
	ExtractStatus string
	EmbedStatus   string
	ExtractedText string
}

// Service 封装文件处理后台流水线与状态查询能力。
type Service struct {
	cfg              *config.Runtime
	repo             repository.FileProcessingRepository
	statusRepo       repository.FileProcessingStatusRepository
	cache            repository.FileProcessingQueueRepository
	extractSvc       *extraction.Service
	embeddingSvc     *appembedding.Service
	logger           *zap.Logger
	extractorVersion string
	reprocessLocks   sync.Map
	fallbackSlots    chan struct{}
}

// NewService 创建文件处理服务。
func NewService(
	cfg config.Config,
	repo repository.FileProcessingRepository,
	cache repository.FileProcessingQueueRepository,
	extractSvc *extraction.Service,
	embeddingSvc *appembedding.Service,
	logger *zap.Logger,
	extractorVersion string,
) *Service {
	return NewServiceWithRuntime(Dependencies{Config: config.NewRuntime(cfg), Repository: repo, Cache: cache, ExtractService: extractSvc, EmbeddingService: embeddingSvc, Logger: logger, ExtractorVersion: extractorVersion})
}

type Dependencies struct {
	Config           *config.Runtime
	Repository       repository.FileProcessingRepository
	StatusRepository repository.FileProcessingStatusRepository
	Cache            repository.FileProcessingQueueRepository
	ExtractService   *extraction.Service
	EmbeddingService *appembedding.Service
	Logger           *zap.Logger
	ExtractorVersion string
}

// NewServiceWithRuntime uses explicitly supplied application dependencies.
func NewServiceWithRuntime(deps Dependencies) *Service {
	return &Service{cfg: deps.Config, repo: deps.Repository, statusRepo: deps.StatusRepository, cache: deps.Cache,
		extractSvc: deps.ExtractService, embeddingSvc: deps.EmbeddingService,
		logger: deps.Logger, extractorVersion: strings.TrimSpace(deps.ExtractorVersion),
		fallbackSlots: make(chan struct{}, fallbackProcessingConcurrency)}
}

// SubmitFileEmbeddings 将显式向量化任务提交到独立的可恢复队列。
func (s *Service) SubmitFileEmbeddings(ctx context.Context, userID uint, fileIDs []string) (appembedding.TargetedSubmissionResult, error) {
	result := appembedding.TargetedSubmissionResult{SubmittedFileIDs: []string{}, Skipped: []appembedding.TargetedFileSkip{}}
	if s == nil || s.embeddingSvc == nil || s.cache == nil {
		return result, appembedding.ErrEmbeddingServiceNotConfigured
	}
	plan, err := s.embeddingSvc.PlanFiles(ctx, userID, fileIDs)
	if err != nil {
		return result, err
	}
	result.Skipped = append(result.Skipped, plan.Skipped...)
	for _, job := range plan.Jobs {
		queued, queueErr := s.embeddingSvc.QueueTargetedJob(ctx, job)
		if queueErr != nil {
			result.Skipped = append(result.Skipped, appembedding.TargetedFileSkip{FileID: job.FileID, Reason: appembedding.SkipReasonSubmitFailed})
			continue
		}
		if !queued {
			result.Skipped = append(result.Skipped, appembedding.TargetedFileSkip{FileID: job.FileID, Reason: appembedding.SkipReasonProcessing})
			continue
		}
		if err := s.cache.EnqueueFileEmbedding(ctx, job.UserID, job.FileID, job.EmbeddingSignature, job.EmbeddingHost); err == nil {
			result.SubmittedFileIDs = append(result.SubmittedFileIDs, job.FileID)
			continue
		}
		if releaseErr := s.embeddingSvc.FailTargetedJob(ctx, job, appembedding.ErrEmbeddingQueueUnavailable); releaseErr != nil && s.logger != nil {
			zapLogger := s.logger
			_ = zapLogger
		}
		result.Skipped = append(result.Skipped, appembedding.TargetedFileSkip{FileID: job.FileID, Reason: appembedding.SkipReasonQueueBusy})
	}
	return result, nil
}

func (s *Service) ResolveFileVectorizationCapabilities(ctx context.Context, files []domainconversation.FileObject) map[string]appembedding.FileVectorizationCapability {
	if s == nil || s.embeddingSvc == nil {
		capabilities := make(map[string]appembedding.FileVectorizationCapability, len(files))
		for i := range files {
			capabilities[files[i].FileID] = appembedding.FileVectorizationCapability{Reason: "embedding_service_unavailable"}
		}
		return capabilities
	}
	return s.embeddingSvc.ResolveFileVectorizationCapabilities(ctx, files)
}

// StartBackgroundWorkers 启动文件处理后台 worker，ctx 取消时 worker 退出。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if s == nil || s.cache == nil {
		return
	}
	consumerName := "worker-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := s.cache.InitFileProcessingStream(ctx); err != nil && s.logger != nil {
		s.logger.Warn("create_file_processing_group_failed", zap.Error(err))
	}
	go s.runFileProcessingWorker(ctx, consumerName)
}

// InitializeUploadedFile 初始化新上传文件的处理状态。
func (s *Service) InitializeUploadedFile(ctx context.Context, fileObj *domainconversation.FileObject) error {
	if fileObj == nil {
		return nil
	}
	now := time.Now()
	if fileObj.FileCategory == "video" || (fileObj.FileCategory == "image" && !s.snapshot().ExtractImageOCREnabled) {
		fileObj.ProcessingStatus = "ready"
		fileObj.ProcessingReady = true
		fileObj.ExtractStatus = "none"
		ragReason := "image_not_applicable"
		if fileObj.FileCategory == "video" {
			ragReason = "video_not_applicable"
		}
		processingStatus := "ready"
		processingReady := true
		processingErrorCode := ""
		processingErrorMessage := ""
		extractStatus := "none"
		if err := s.repo.UpdateFileObjectProcessing(ctx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
			ExpectedStoragePath:    fileObj.StoragePath,
			ProcessingStatus:       &processingStatus,
			ProcessingReady:        &processingReady,
			ProcessingErrorCode:    &processingErrorCode,
			ProcessingErrorMessage: &processingErrorMessage,
			ExtractStatus:          &extractStatus,
		}); err != nil {
			return err
		}
		return s.repo.UpdateFileObjectProcessingState(ctx, &domainconversation.FileObjectProcessing{
			FileObjectID:        fileObj.ID,
			ExpectedStoragePath: fileObj.StoragePath,
			UserID:              fileObj.UserID,
			DetectedMIME:        fileObj.DetectedMIME,
			FileCategory:        fileObj.FileCategory,
			ProcessingStatus:    "ready",
			ExtractStatus:       "none",
			RAGReady:            false,
			RAGReason:           ragReason,
			ExtractorVersion:    s.version(),
			StartedAt:           &now,
			CompletedAt:         &now,
		})
	}

	if !supportsExtraction(fileObj.FileCategory) {
		return s.markFileProcessingFailed(ctx, fileObj, "mime_blocked", "unsupported file category")
	}

	processingStatus := "queued"
	processingReady := false
	extractStatus := "none"
	if err := s.repo.UpdateFileObjectProcessing(ctx, fileObj.UserID, fileObj.FileID, repository.UpdateFileObjectProcessingInput{
		ExpectedStoragePath: fileObj.StoragePath,
		ProcessingStatus:    &processingStatus,
		ProcessingReady:     &processingReady,
		ExtractStatus:       &extractStatus,
	}); err != nil {
		return err
	}
	if err := s.repo.UpdateFileObjectProcessingState(ctx, &domainconversation.FileObjectProcessing{
		FileObjectID:        fileObj.ID,
		ExpectedStoragePath: fileObj.StoragePath,
		UserID:              fileObj.UserID,
		DetectedMIME:        fileObj.DetectedMIME,
		FileCategory:        fileObj.FileCategory,
		ProcessingStatus:    "queued",
		ExtractStatus:       "none",
		ExtractorVersion:    s.version(),
		StartedAt:           &now,
	}); err != nil {
		return err
	}
	return s.enqueueFileProcessing(ctx, fileObj.UserID, fileObj.FileID, 0, "")
}

// ProcessFile 执行单个文件处理任务。
func (s *Service) ProcessFile(ctx context.Context, userID uint, fileID string) error {
	_, err := s.processFile(ctx, userID, fileID, false, "")
	return err
}

func (s *Service) processFile(ctx context.Context, userID uint, fileID string, allowRecovery bool, attemptID string) (bool, error) {
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil || fileObj == nil {
		return false, err
	}
	if fileObj.ProcessingStatus == "ready" || fileObj.ProcessingStatus == "failed" {
		return false, nil
	}
	if attemptID == "" {
		return false, nil
	}
	claimed, err := s.repo.TryClaimFileObjectProcessing(ctx, userID, fileID, allowRecovery, s.version(), attemptID)
	if err != nil || !claimed {
		return false, err
	}
	if fileObj.FileCategory == "image" && !s.snapshot().ExtractImageOCREnabled {
		return true, s.updateClaimedFileProcessingState(ctx, attemptID, s.readyWithoutExtractionState(fileObj, "image_not_applicable"))
	}

	cfg := s.snapshot()
	extractTimeout := resolveProcessingExtractTimeout(cfg, fileObj.FileCategory)
	runCtx, cancel := context.WithTimeout(ctx, extractTimeout+fixedEmbeddingTimeout)
	defer cancel()

	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	fileObj.ProcessingStartedAt = &startedAt

	extractCtx, extractCancel := context.WithTimeout(runCtx, extractTimeout)
	extractResult, extractErr := s.extractTextForProcessing(extractCtx, *fileObj)
	extractCancel()
	if extractErr != nil {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if extraction.IsEmptyContent(extractErr) {
			return true, s.markClaimedFileProcessingEmpty(runCtx, fileObj, attemptID, extraction.ErrorCode(extractErr))
		}
		code, message := resolveProcessingFailure(fileObj, extractErr)
		return true, s.markClaimedFileProcessingFailed(runCtx, fileObj, attemptID, code, message)
	}
	if strings.TrimSpace(extractResult.Text) == "" {
		return true, s.markClaimedFileProcessingEmpty(runCtx, fileObj, attemptID, domainconversation.FileErrorCodeNoExtractableText)
	}

	extractPath, err := s.extractSvc.WriteExtractedText(runCtx, fileObj.UserID, fileObj.FileID, fileObj.StoragePath, startedAt, extractResult.Text)
	if err != nil {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		return true, s.markClaimedFileProcessingFailed(runCtx, fileObj, attemptID, "extract_failed", HumanizeFileProcessingError(fileObj.FileCategory, "extract_failed", ""))
	}
	return true, s.processClaimedFile(runCtx, fileObj, attemptID, extractResult, extractPath)
}

func (s *Service) readyWithoutExtractionState(fileObj *domainconversation.FileObject, ragReason string) *domainconversation.FileObjectProcessing {
	now := time.Now()
	return &domainconversation.FileObjectProcessing{
		FileObjectID:     fileObj.ID,
		UserID:           fileObj.UserID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: "ready",
		ProcessingReady:  true,
		ExtractStatus:    "none",
		RAGReady:         false,
		RAGReason:        ragReason,
		ExtractorVersion: s.version(),
		StartedAt:        &now,
		CompletedAt:      &now,
	}
}

func (s *Service) processClaimedFile(ctx context.Context, fileObj *domainconversation.FileObject, attemptID string, extractResult extraction.Result, extractPath string) error {
	runCtx := ctx
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	if fileObj.ProcessingStartedAt != nil {
		startedAt = *fileObj.ProcessingStartedAt
	}
	now := time.Now()
	preview := compactSnippet(extractResult.Text, defaultProcessingPreview)
	ragAvailable, ragReason := s.embeddingSvc.Available(runCtx)
	indexingAvailable, _ := s.embeddingSvc.IndexingAvailable(runCtx)
	resultRAGReady := false
	resultRAGReason := "not_applicable"
	if supportsRAG(fileObj.FileCategory) {
		resultRAGReady = ragAvailable
		if ragAvailable {
			resultRAGReason = "embedding_pending"
		} else {
			resultRAGReason = ragReason
		}
	}
	shouldEmbed := indexingAvailable && supportsRAG(fileObj.FileCategory) && s.embeddingSvc.ShouldTrigger(*fileObj)
	nextProcessingStatus := "ready"
	if shouldEmbed {
		nextProcessingStatus = "embedding"
	}
	if err := s.updateClaimedFileProcessingState(runCtx, attemptID, &domainconversation.FileObjectProcessing{
		FileObjectID:      fileObj.ID,
		UserID:            fileObj.UserID,
		DetectedMIME:      fileObj.DetectedMIME,
		FileCategory:      fileObj.FileCategory,
		ProcessingStatus:  nextProcessingStatus,
		ProcessingReady:   true,
		ExtractStatus:     "ready",
		ExtractEngine:     extractResult.Engine,
		ExtractChars:      len([]rune(extractResult.Text)),
		ExtractPages:      extractResult.PageCount,
		PageCount:         extractResult.PageCount,
		PreviewText:       preview,
		OCRUsed:           extractResult.OCRUsed,
		RAGReady:          resultRAGReady,
		RAGReason:         resultRAGReason,
		ExtractorVersion:  s.version(),
		StartedAt:         &startedAt,
		CompletedAt:       &now,
		ExtractedAt:       &now,
	}); err != nil {
		if errors.Is(err, errFileProcessingClaimLost) {
			s.removeOrphanExtract(runCtx, fileObj.ID, fileObj.UserID, fileObj.FileID, extractPath)
		}
		return err
	}
	if shouldEmbed {
		embedCtx, embedCancel := context.WithTimeout(runCtx, fixedEmbeddingTimeout)
		embedErr := s.embeddingSvc.ProcessFile(embedCtx, *fileObj)
		embedCancel()
		if embedErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return s.updateClaimedFileProcessingState(runCtx, attemptID, &domainconversation.FileObjectProcessing{
				FileObjectID:      fileObj.ID,
				UserID:            fileObj.UserID,
				DetectedMIME:      fileObj.DetectedMIME,
				FileCategory:      fileObj.FileCategory,
				ProcessingStatus:  "ready",
				ProcessingReady:   true,
				ExtractStatus:     "ready",
				ExtractEngine:     extractResult.Engine,
				ExtractChars:      len([]rune(extractResult.Text)),
				ExtractPages:      extractResult.PageCount,
				PageCount:         extractResult.PageCount,
				PreviewText:       preview,
				OCRUsed:           extractResult.OCRUsed,
				RAGReady:          false,
				RAGReason:         "embed_failed",
				ErrorCode:         "embed_failed",
				ErrorMessage:      appembedding.ErrorSummary(embedErr),
				ExtractorVersion:  s.version(),
				StartedAt:         &startedAt,
				CompletedAt:       &now,
				ExtractedAt:       &now,
			})
		}
	}

	return s.updateClaimedFileProcessingState(runCtx, attemptID, &domainconversation.FileObjectProcessing{
		FileObjectID:      fileObj.ID,
		UserID:            fileObj.UserID,
		DetectedMIME:      fileObj.DetectedMIME,
		FileCategory:      fileObj.FileCategory,
		ProcessingStatus:  "ready",
		ProcessingReady:   true,
		ExtractStatus:     "ready",
		ExtractEngine:     extractResult.Engine,
		ExtractChars:      len([]rune(extractResult.Text)),
		ExtractPages:      extractResult.PageCount,
		PageCount:         extractResult.PageCount,
		PreviewText:       preview,
		OCRUsed:           extractResult.OCRUsed,
		RAGReady:          ragAvailable && supportsRAG(fileObj.FileCategory),
		RAGReason: func() string {
			if !supportsRAG(fileObj.FileCategory) {
				return "not_applicable"
			}
			if ragAvailable {
				return "ready"
			}
			return ragReason
		}(),
		ExtractorVersion: s.version(),
		StartedAt:        &startedAt,
		CompletedAt:      &now,
		ExtractedAt:      &now,
	})
}

// removeOrphanExtract 在版本冲突（文件内容已被其他任务覆盖）时清理本次未提交的
// 提取对象；对象仍被其他保留行（如克隆文件）引用时不做删除。
func (s *Service) removeOrphanExtract(ctx context.Context, fileObjID uint, userID uint, fileID string, extractPath string) {
	if strings.TrimSpace(extractPath) == "" {
		return
	}
	removable, err := s.repo.CanRemoveExtractStoragePath(ctx, fileObjID, userID, extractPath)
	if err != nil || !removable {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failurePersistTimeout)
	defer cancel()
	if err := s.extractSvc.DeleteExtractedText(cleanupCtx, extractPath); err != nil && s.logger != nil {
		s.logger.Warn("remove_uncommitted_extract_failed",
			zap.String("file_id", fileID),
			zap.Error(err),
		)
	}
}

// GetFileProcessingStatus 查询文件处理状态。
func (s *Service) GetFileProcessingStatus(ctx context.Context, userID uint, fileID string) (*FileProcessingStatusDTO, error) {
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	if fileObj == nil {
		return nil, ErrFileNotFound
	}
	result := fileProcessingStatusFromFileObject(fileObj)
	if s.statusRepo != nil {
		if statuses, statusesErr := s.statusRepo.GetActiveFileProcessingStatusesByIDs(ctx, userID, []string{fileObj.FileID}); statusesErr == nil && len(statuses) == 1 {
			result = fileProcessingStatusFromFileObject(&statuses[0])
		}
	}
	if capability, ok := s.ResolveFileVectorizationCapabilities(ctx, []domainconversation.FileObject{*fileObj})[fileObj.FileID]; ok {
		result.CanVectorize = capability.CanVectorize
		result.VectorizationReason = capability.Reason
	}
	return &result, nil
}

// GetFileProcessingStatuses 批量查询文件处理状态。
func (s *Service) GetFileProcessingStatuses(ctx context.Context, userID uint, fileIDs []string) ([]FileProcessingStatusDTO, error) {
	normalizedIDs := make([]string, 0, len(fileIDs))
	seen := make(map[string]struct{}, len(fileIDs))
	for _, value := range fileIDs {
		fileID := strings.TrimSpace(value)
		if fileID == "" {
			continue
		}
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		normalizedIDs = append(normalizedIDs, fileID)
	}
	if len(normalizedIDs) == 0 {
		return []FileProcessingStatusDTO{}, nil
	}
	statusRepo := s.statusRepo
	if statusRepo == nil {
		if fallback, ok := any(s.repo).(repository.FileProcessingStatusRepository); ok {
			statusRepo = fallback
		}
	}
	if statusRepo == nil {
		return []FileProcessingStatusDTO{}, nil
	}
	fileObjects, err := statusRepo.GetActiveFileProcessingStatusesByIDs(ctx, userID, normalizedIDs)
	if err != nil {
		return nil, err
	}
	filesByID := make(map[string]domainconversation.FileObject, len(fileObjects))
	for _, fileObj := range fileObjects {
		filesByID[fileObj.FileID] = fileObj
	}
	capabilities := s.ResolveFileVectorizationCapabilities(ctx, fileObjects)
	results := make([]FileProcessingStatusDTO, 0, len(normalizedIDs))
	for _, fileID := range normalizedIDs {
		fileObj, ok := filesByID[fileID]
		if !ok {
			continue
		}
		dto := fileProcessingStatusFromFileObject(&fileObj)
		if capability, ok := capabilities[fileID]; ok {
			dto.CanVectorize = capability.CanVectorize
			dto.VectorizationReason = capability.Reason
		}
		results = append(results, dto)
	}
	return results, nil
}

func fileProcessingStatusFromFileObject(fileObj *domainconversation.FileObject) FileProcessingStatusDTO {
	dto := FileProcessingStatusDTO{
		FileID:           fileObj.FileID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: fileObj.ProcessingStatus,
		ProcessingReady:  fileObj.ProcessingReady,
		ExtractStatus:    fileObj.ExtractStatus,
		EmbedStatus:      fileObj.EmbedStatus,
		PreviewText:      fileObj.PreviewText,
		OCRUsed:          fileObj.OCRUsed,
		RAGReady:         fileObj.RAGReady,
		RAGReason:        fileObj.RAGReason,
		ErrorCode:        fileObj.ProcessingErrorCode,
		ErrorMessage:     fileObj.ProcessingErrorMessage,
		ExtractChars:     fileObj.ExtractChars,
		ExtractPages:     fileObj.ExtractPages,
		ChunkCount:       fileObj.ChunkCount,
		EmbedError:       fileObj.EmbedError,
		StartedAt:        fileObj.ProcessingStartedAt,
		CompletedAt:      fileObj.ProcessingCompletedAt,
		UpdatedAt:        fileObj.UpdatedAt,
	}
	dto.ErrorMessage = HumanizeFileProcessingError(dto.FileCategory, dto.ErrorCode, dto.ErrorMessage)
	return dto
}

// WaitUntilReady 等待文件处理完成，并在就绪时返回提取产物。
func (s *Service) WaitUntilReady(
	ctx context.Context,
	userID uint,
	fileID string,
	onProgress func(fileObj *domainconversation.FileObject),
) (*ReadyFileResult, error) {
	if err := s.EnsureImageOCRProcessing(ctx, userID, fileID); err != nil {
		return nil, err
	}
	for {
		fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
		if err != nil || fileObj == nil {
			return nil, err
		}
		if fileObj.ProcessingReady {
			result := &ReadyFileResult{
				File:          *fileObj,
				DetectedMIME:  fileObj.DetectedMIME,
				FileCategory:  fileObj.FileCategory,
				PageCount:     fileObj.PageCount,
				ExtractStatus: fileObj.ExtractStatus,
				EmbedStatus:   fileObj.EmbedStatus,
			}
			if processingResult, resultErr := s.repo.GetFileObjectProcessingByObjectID(ctx, fileObj.ID); resultErr == nil && processingResult != nil && strings.TrimSpace(processingResult.ExtractStoragePath) != "" && s.extractSvc != nil {
				if text, readErr := s.extractSvc.ReadExtractedText(ctx, processingResult.ExtractStoragePath); readErr == nil {
					result.ExtractedText = text
				}
			}
			return result, nil
		}
		if fileObj.ProcessingStatus == "failed" {
			if onProgress != nil {
				onProgress(fileObj)
			}
			message := HumanizeFileProcessingError(fileObj.FileCategory, fileObj.ProcessingErrorCode, fileObj.ProcessingErrorMessage)
			if message == "" {
				message = fileObj.ProcessingStatus
			}
			return nil, fmt.Errorf("%w: %s", ErrFileProcessingFailed, message)
		}
		if onProgress != nil {
			onProgress(fileObj)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// EnsureImageOCRProcessing 为启用 OCR 后仍处于旧 ready/none 状态的图片安全地重新入队。
func (s *Service) EnsureImageOCRProcessing(ctx context.Context, userID uint, fileID string) error {
	return s.ensureImageOCRProcessing(ctx, userID, fileID)
}

func (s *Service) ensureImageOCRProcessing(ctx context.Context, userID uint, fileID string) error {
	if !s.snapshot().ExtractImageOCREnabled {
		return nil
	}
	key := fmt.Sprintf("%d:%s", userID, strings.TrimSpace(fileID))
	if strings.HasSuffix(key, ":") {
		return nil
	}
	lockValue, _ := s.reprocessLocks.LoadOrStore(key, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	defer s.reprocessLocks.Delete(key)

	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil || fileObj == nil {
		return err
	}
	if fileObj.FileCategory != "image" ||
		!fileObj.ProcessingReady ||
		!strings.EqualFold(strings.TrimSpace(fileObj.ExtractStatus), "none") {
		return nil
	}
	return s.InitializeUploadedFile(ctx, fileObj)
}

func (s *Service) handleEmbeddingMessage(ctx context.Context, consumerName string, msg repository.FileProcessingMessage) {
	if s.embeddingSvc == nil {
		return
	}
	job := appembedding.TargetedJob{FileID: msg.FileID, UserID: msg.UserID, EmbeddingSignature: msg.EmbeddingSignature, EmbeddingHost: msg.EmbeddingHost}
	processingCtx, cancelProcessing := context.WithCancel(ctx)
	defer cancelProcessing()
	err := s.embeddingSvc.ProcessTargetedJob(processingCtx, job)
	if taskCtxErr := ctx.Err(); taskCtxErr != nil {
		return
	}
	if owned, ownershipErr := s.cache.RenewFileProcessingMessageLease(ctx, consumerName, msg); ownershipErr != nil || !owned {
		return
	}
	if err == nil {
		_, _ = s.cache.SettleFileProcessingMessage(ctx, consumerName, msg)
		return
	}
	failureMessage := appembedding.ErrorSummary(err)
	if msg.Retry < fileProcessingMaxRetries {
		if retryStateErr := s.embeddingSvc.RequeueTargetedJob(ctx, job, err); retryStateErr != nil {
			return
		}
		_, _ = s.cache.RequeueFileProcessingMessage(ctx, consumerName, msg, msg.Retry+1, failureMessage)
		return
	}
	if failErr := s.embeddingSvc.FailTargetedJob(ctx, job, err); failErr != nil {
		return
	}
	_, _ = s.cache.DeadLetterFileProcessingMessage(ctx, consumerName, msg, failureMessage)
}

func (s *Service) runFileProcessingWorker(ctx context.Context, consumerName string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		claimed, claimErr := s.cache.ClaimTimedOutFileProcessingMessages(ctx, consumerName)
		if claimErr != nil {
			if ctx.Err() != nil {
				return
			}
			if s.logger != nil {
				s.logger.Warn("file_processing_worker_claim_failed", zap.Error(claimErr))
			}
		} else if len(claimed) > 0 {
			for _, msg := range claimed {
				s.handleProcessingMessage(ctx, consumerName, msg)
			}
			continue
		}

		messages, err := s.cache.ReadFileProcessingMessages(ctx, consumerName)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if s.logger != nil {
				s.logger.Warn("file_processing_worker_read_failed", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		for _, msg := range messages {
			s.handleProcessingMessage(ctx, consumerName, msg)
		}
	}
}

type fileMessageReader func(context.Context, string) ([]repository.FileProcessingMessage, error)

type fileMessageHandler func(context.Context, string, repository.FileProcessingMessage)

func (s *Service) handleProcessingMessage(ctx context.Context, consumerName string, msg repository.FileProcessingMessage) {
	if strings.TrimSpace(msg.FileID) == "" {
		s.settleProcessingMessage(ctx, consumerName, msg)
		return
	}
	if msg.Kind == repository.FileProcessingKindEmbedding {
		s.handleEmbeddingMessage(ctx, consumerName, msg)
		return
	}

	attemptID := uuid.NewString()
	processingCtx, cancelProcessing := context.WithCancel(ctx)
	defer cancelProcessing()
	leaseCtx, stopLease := context.WithCancel(ctx)
	leaseDone := make(chan struct{})
	ownershipLost := make(chan struct{})
	go s.renewProcessingMessageLease(leaseCtx, leaseDone, ownershipLost, cancelProcessing, consumerName, msg)
	claimed, processErr := s.processFile(processingCtx, msg.UserID, msg.FileID, msg.Reclaimed, attemptID)
	stopLease()
	<-leaseDone
	if ctx.Err() != nil {
		return
	}
	select {
	case <-ownershipLost:
		return
	default:
	}
	if owned, ownershipErr := s.cache.RenewFileProcessingMessageLease(ctx, consumerName, msg); ownershipErr != nil || !owned {
		if ownershipErr != nil && s.logger != nil {
			s.logger.Warn("verify_file_processing_lease_failed",
				zap.Uint("user_id", msg.UserID),
				zap.String("file_id", msg.FileID),
				zap.String("message_id", msg.ID),
				zap.Error(ownershipErr),
			)
		}
		return
	}
	if processErr != nil {
		if errors.Is(processErr, errFileProcessingClaimLost) {
			s.settleProcessingMessage(ctx, consumerName, msg)
			return
		}
		if !claimed {
			if s.logger != nil {
				s.logger.Warn("prepare_file_processing_failed",
					zap.Uint("user_id", msg.UserID),
					zap.String("file_id", msg.FileID),
					zap.String("message_id", msg.ID),
					zap.Error(processErr),
				)
			}
			return
		}
		failureMessage := processingErrorSummary(processErr)
		if msg.Retry < fileProcessingMaxRetries {
			if reset, resetErr := s.repo.ResetFileObjectProcessingForRetry(ctx, msg.UserID, msg.FileID, attemptID); resetErr != nil || !reset {
				if s.logger != nil {
					s.logger.Warn("reset_file_processing_for_retry_failed",
						zap.Uint("user_id", msg.UserID),
						zap.String("file_id", msg.FileID),
						zap.Int("retry", msg.Retry),
						zap.Error(resetErr),
					)
				}
				return
			}
			if settled, requeueErr := s.cache.RequeueFileProcessingMessage(ctx, consumerName, msg, msg.Retry+1, failureMessage); requeueErr != nil || !settled {
				if s.logger != nil {
					s.logger.Warn("requeue_file_processing_failed",
						zap.Uint("user_id", msg.UserID),
						zap.String("file_id", msg.FileID),
						zap.Int("retry", msg.Retry),
						zap.Error(requeueErr),
					)
				}
				return
			}
		} else {
			if finalizeErr := s.forceFinalizeFailed(ctx, msg.UserID, msg.FileID, attemptID, processErr); finalizeErr != nil {
				if s.logger != nil {
					s.logger.Warn("force_finalize_file_processing_failed",
						zap.Uint("user_id", msg.UserID),
						zap.String("file_id", msg.FileID),
						zap.Error(finalizeErr),
					)
				}
				return
			}
			if !s.deadLetterProcessingMessage(ctx, consumerName, msg) {
				return
			}
		}
		if s.logger != nil {
			s.logger.Warn("process_queued_file_failed",
				zap.Uint("user_id", msg.UserID),
				zap.String("file_id", msg.FileID),
				zap.Int("retry", msg.Retry),
				zap.Error(processErr),
			)
		}
		return
	}

	if !claimed && msg.Retry >= fileProcessingMaxRetries && strings.TrimSpace(msg.LastError) != "" {
		if fileObj, lookupErr := s.repo.GetActiveFileObjectByID(ctx, msg.UserID, msg.FileID); lookupErr == nil && fileObj != nil && fileObj.ProcessingStatus == "failed" {
			s.deadLetterProcessingMessage(ctx, consumerName, msg)
			return
		}
	}
	s.settleProcessingMessage(ctx, consumerName, msg)
}

func (s *Service) renewProcessingMessageLease(ctx context.Context, done chan<- struct{}, ownershipLost chan<- struct{}, cancelProcessing context.CancelFunc, consumerName string, msg repository.FileProcessingMessage) {
	defer close(done)
	ticker := time.NewTicker(fileProcessingLeaseRenew)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			owned, err := s.cache.RenewFileProcessingMessageLease(ctx, consumerName, msg)
			if err == nil && owned {
				continue
			}
			if err != nil && s.logger != nil {
				s.logger.Warn("file_processing_lease_failed", zap.Error(err))
			}
			close(ownershipLost)
			cancelProcessing()
			return
		}
	}
}

func (s *Service) settleProcessingMessage(ctx context.Context, consumerName string, msg repository.FileProcessingMessage) {
	if settled, err := s.cache.SettleFileProcessingMessage(ctx, consumerName, msg); (err != nil || !settled) && s.logger != nil {
		s.logger.Warn("settle_file_processing_message_failed",
			zap.Uint("user_id", msg.UserID),
			zap.String("file_id", msg.FileID),
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
	}
}

func (s *Service) deadLetterProcessingMessage(ctx context.Context, consumerName string, msg repository.FileProcessingMessage) bool {
	if settled, err := s.cache.DeadLetterFileProcessingMessage(ctx, consumerName, msg, processingQueueFailureMessage); err == nil && settled {
		return true
	} else if s.logger != nil {
		s.logger.Warn("dead_letter_file_processing_failed",
			zap.Uint("user_id", msg.UserID),
			zap.String("file_id", msg.FileID),
			zap.Int("retry", msg.Retry),
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
	}
	return false
}

func (s *Service) renewProcessingLease(ctx context.Context, consumerName string, msg repository.FileProcessingMessage) bool {
	leaseCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	owned, err := s.cache.RenewFileProcessingMessageLease(leaseCtx, consumerName, msg)
	if err != nil && s.logger != nil {
		s.logger.Warn("file_processing_lease_failed", zap.Error(err))
	}
	return err == nil && owned
}

func (s *Service) forceFinalizeFailed(parent context.Context, userID uint, fileID string, attemptID string, processingErr error) error {
	if s == nil || s.repo == nil || strings.TrimSpace(fileID) == "" {
		return nil
	}
	ctx, cancel := background.WithTimeout(parent, failurePersistTimeout)
	defer cancel()

	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil {
		return err
	}
	if fileObj == nil {
		return nil
	}
	if fileObj.ProcessingStatus == "ready" || fileObj.ProcessingStatus == "failed" {
		return nil
	}

	code, message := resolveProcessingFailure(fileObj, processingErr)
	if err := s.markClaimedFileProcessingFailed(ctx, fileObj, attemptID, code, message); errors.Is(err, errFileProcessingClaimLost) {
		return s.markFileProcessingFailed(ctx, fileObj, code, message)
	} else {
		return err
	}
}

func (s *Service) enqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if s.cache == nil {
		return s.processInFallbackMode(ctx, userID, fileID)
	}
	if strings.TrimSpace(lastError) == "" {
		lastError = processingQueueFailureMessage
	}
	return s.cache.EnqueueFileProcessing(ctx, userID, fileID, retry, lastError)
}

func (s *Service) processInFallbackMode(parent context.Context, userID uint, fileID string) error {
	select {
	case s.fallbackSlots <- struct{}{}:
	default:
		return repository.ErrFileProcessingQueueFull
	}
	background.Go(s.logger, "fallback_file_processing", func() {
		defer func() { <-s.fallbackSlots }()
		attemptID := uuid.NewString()
		taskCtx, cancel := background.WithTimeout(parent, fallbackProcessingTimeout)
		defer cancel()
		claimed, err := s.processFile(taskCtx, userID, fileID, false, attemptID)
		if err == nil {
			return
		}
		if claimed || taskCtx.Err() != nil {
			_ = s.forceFinalizeFailed(taskCtx, userID, fileID, attemptID, err)
		}
		if s.logger != nil {
			s.logger.Warn("fallback_file_processing_failed",
				zap.Uint("user_id", userID),
				zap.String("file_id", fileID),
				zap.Error(err),
			)
		}
	})
	return nil
}

// EnqueueFileProcessing 将 pending 文件原子占用并提交到处理队列。
func (s *Service) EnqueueFileProcessing(ctx context.Context, userID uint, fileID string) error {
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
	if err != nil || fileObj == nil {
		return err
	}
	claimed, err := s.repo.ClaimFileProcessingQueue(ctx, userID, fileID, fileObj.StoragePath)
	if err != nil || !claimed {
		return err
	}
	if err := s.enqueueFileProcessing(ctx, userID, fileID, 0, ""); err != nil {
		s.releaseFileProcessingQueueClaim(ctx, userID, fileID, fileObj.StoragePath)
		return err
	}
	return nil
}

func (s *Service) releaseFileProcessingQueueClaim(ctx context.Context, userID uint, fileID string, storagePath string) {
	writeCtx := context.Background()
	if ctx != nil {
		writeCtx = context.WithoutCancel(ctx)
	}
	writeCtx, cancel := context.WithTimeout(writeCtx, failurePersistTimeout)
	defer cancel()
	_ = s.repo.ReleaseFileProcessingQueueClaim(writeCtx, userID, fileID, storagePath)
}

// RecoverFileProcessingQueue 补提交在重启或队列故障窗口中遗留的文件处理任务。
func (s *Service) RecoverFileProcessingQueue(ctx context.Context, debounce time.Duration) error {
	if debounce <= 0 {
		debounce = time.Minute
	}
	const batchSize = 100
	now := time.Now()
	files, err := s.repo.ClaimRecoverableFilesForProcessing(
		ctx,
		now.Add(-debounce),
		now.Add(-2*time.Minute),
		now.Add(-2*time.Hour),
		batchSize,
	)
	if err != nil {
		return err
	}
	var recoveryErr error
	for i := range files {
		fileObj := files[i]
		if err := s.enqueueFileProcessing(ctx, fileObj.UserID, fileObj.FileID, 0, "recovered pending file processing"); err != nil {
			recoveryErr = errors.Join(recoveryErr, err)
			s.releaseFileProcessingQueueClaim(ctx, fileObj.UserID, fileObj.FileID, fileObj.StoragePath)
		}
	}
	return recoveryErr
}

func (s *Service) markClaimedFileProcessingEmpty(ctx context.Context, fileObj *domainconversation.FileObject, attemptID string, reason string) error {
	if fileObj == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(context.Background(), failurePersistTimeout)
		defer cancel()
	}
	now := time.Now()
	return s.updateClaimedFileProcessingState(writeCtx, attemptID, &domainconversation.FileObjectProcessing{
		FileObjectID:     fileObj.ID,
		UserID:           fileObj.UserID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: "ready",
		ProcessingReady:  false,
		ExtractStatus:    domainconversation.FileSubprocessStatusEmpty,
		RAGReady:         false,
		RAGReason:        reason,
		ExtractorVersion: s.version(),
		CompletedAt:      &now,
	})
}

func (s *Service) updateClaimedFileProcessingState(ctx context.Context, attemptID string, state *domainconversation.FileObjectProcessing) error {
	updated, err := s.repo.UpdateClaimedFileObjectProcessingState(ctx, state, attemptID)
	if err != nil {
		return err
	}
	if !updated {
		return errFileProcessingClaimLost
	}
	return nil
}

func (s *Service) markFileProcessingFailed(ctx context.Context, fileObj *domainconversation.FileObject, code string, message string) error {
	if fileObj == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = background.WithTimeout(ctx, failurePersistTimeout)
		defer cancel()
	}
	return s.repo.UpdateFileObjectProcessingState(
		writeCtx,
		s.failedFileProcessingState(fileObj, code, message),
	)
}

func (s *Service) markClaimedFileProcessingFailed(ctx context.Context, fileObj *domainconversation.FileObject, attemptID string, code string, message string) error {
	if fileObj == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = background.WithTimeout(ctx, failurePersistTimeout)
		defer cancel()
	}
	return s.updateClaimedFileProcessingState(
		writeCtx,
		attemptID,
		s.failedFileProcessingState(fileObj, code, message),
	)
}

func (s *Service) failedFileProcessingState(fileObj *domainconversation.FileObject, code string, message string) *domainconversation.FileObjectProcessing {
	now := time.Now()
	return &domainconversation.FileObjectProcessing{
		FileObjectID:     fileObj.ID,
		UserID:           fileObj.UserID,
		DetectedMIME:     fileObj.DetectedMIME,
		FileCategory:     fileObj.FileCategory,
		ProcessingStatus: "failed",
		ProcessingReady:  false,
		ExtractStatus:    "failed",
		RAGReady:         false,
		RAGReason:        code,
		ErrorCode:        code,
		ErrorMessage:     textutil.TruncateTrimmed(HumanizeFileProcessingError(fileObj.FileCategory, code, message), 255),
		ExtractorVersion: s.version(),
		StartedAt:        fileObj.ProcessingStartedAt,
		CompletedAt:      &now,
	}
}

func (s *Service) extractTextForProcessing(ctx context.Context, fileObj domainconversation.FileObject) (extraction.Result, error) {
	if s == nil || s.extractSvc == nil {
		return extraction.Result{}, errExtractionServiceNotConfigured
	}
	type extractOutcome struct {
		result extraction.Result
		err    error
	}

	done := make(chan extractOutcome, 1)
	go func() {
		result, err := s.extractSvc.ExtractStoredFile(ctx, extraction.ExtractInput{
			File:                  fileObj,
			PDFMaxPages:           0,
			OCREngine:             s.snapshot().ExtractOCREngine,
			ImageOCREnabled:       s.snapshot().ExtractImageOCREnabled,
			PDFOCRFallbackEnabled: s.snapshot().ExtractPDFOCRFallbackEnabled,
		})
		done <- extractOutcome{result: result, err: err}
	}()

	select {
	case <-ctx.Done():
		return extraction.Result{}, ctx.Err()
	case outcome := <-done:
		return outcome.result, outcome.err
	}
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

func resolveProcessingExtractTimeout(cfg config.Config, fileCategory string) time.Duration {
	primaryTimeout := resolvePrimaryExtractTimeout(cfg)
	ocrTimeout := resolveOCRExtractTimeout(cfg)

	switch strings.ToLower(strings.TrimSpace(fileCategory)) {
	case "image":
		if cfg.ExtractImageOCREnabled {
			return ocrTimeout
		}
	case "pdf":
		if cfg.ExtractPDFOCRFallbackEnabled {
			return primaryTimeout + ocrTimeout
		}
	}
	return primaryTimeout
}

func resolvePrimaryExtractTimeout(cfg config.Config) time.Duration {
	timeoutSeconds := 0
	switch strings.ToLower(strings.TrimSpace(cfg.ExtractEngine)) {
	case extraction.EngineTika:
		timeoutSeconds = cfg.ExtractTikaTimeoutSeconds
	case extraction.EngineDocling:
		timeoutSeconds = cfg.ExtractDoclingTimeoutSeconds
	case extraction.EngineMinerU:
		timeoutSeconds = cfg.ExtractMinerUTimeoutSeconds
	default:
		timeoutSeconds = int(defaultExtractTimeout / time.Second)
	}
	if timeoutSeconds <= 0 {
		return defaultExtractTimeout
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func resolveOCRExtractTimeout(cfg config.Config) time.Duration {
	timeoutSeconds := 0
	switch strings.ToLower(strings.TrimSpace(cfg.ExtractOCREngine)) {
	case extraction.OCREngineTesseract:
		timeoutSeconds = cfg.ExtractTesseractOCRTimeoutSeconds
	case extraction.OCREngineRapidOCR:
		timeoutSeconds = cfg.ExtractRapidOCRTimeoutSeconds
	case extraction.OCREnginePaddle:
		timeoutSeconds = cfg.ExtractPaddleOCRTimeoutSeconds
	case extraction.OCREngineTencent:
		timeoutSeconds = cfg.ExtractTencentOCRTimeoutSeconds
	case extraction.OCREngineAliyun:
		timeoutSeconds = cfg.ExtractAliyunOCRTimeoutSeconds
	case extraction.OCREngineMistral:
		timeoutSeconds = cfg.ExtractMistralOCRTimeoutSeconds
	case extraction.OCREngineLLM:
		timeoutSeconds = cfg.ExtractLLMOCRTimeoutSeconds
	case extraction.OCREngineSystemVision:
		timeoutSeconds = cfg.MultimodalDelegationTimeoutSeconds
	default:
		timeoutSeconds = int(defaultExtractTimeout / time.Second)
	}
	if timeoutSeconds <= 0 {
		return defaultExtractTimeout
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func (s *Service) version() string {
	if strings.TrimSpace(s.extractorVersion) == "" {
		return DefaultExtractorVersion
	}
	return s.extractorVersion
}

func classifyProcessingErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "tesseract_ocr_disabled"):
		return "tesseract_ocr_disabled"
	case strings.Contains(msg, "tesseract_ocr_failed"):
		return "tesseract_ocr_failed"
	case strings.Contains(msg, "tesseract_ocr_empty_content"):
		return "tesseract_ocr_empty_content"
	case strings.Contains(msg, "tesseract_ocr_unprocessable"):
		return "tesseract_ocr_unprocessable"
	case strings.Contains(msg, "tesseract_ocr_invalid_response"):
		return "tesseract_ocr_invalid_response"
	case strings.Contains(msg, "tesseract_ocr_unauthorized"):
		return "tesseract_ocr_unauthorized"
	case strings.Contains(msg, "tesseract_ocr_forbidden"):
		return "tesseract_ocr_forbidden"
	case strings.Contains(msg, "tesseract_ocr_http_"):
		return "tesseract_ocr_http_error"
	case strings.Contains(msg, "tesseract_ocr_unavailable"):
		return "tesseract_ocr_unavailable"
	case strings.Contains(msg, "rapidocr_ocr_disabled"):
		return "rapidocr_ocr_disabled"
	case strings.Contains(msg, "rapidocr_ocr_failed"):
		return "rapidocr_ocr_failed"
	case strings.Contains(msg, "rapidocr_ocr_empty_content"):
		return "rapidocr_ocr_empty_content"
	case strings.Contains(msg, "rapidocr_ocr_unprocessable"):
		return "rapidocr_ocr_unprocessable"
	case strings.Contains(msg, "rapidocr_ocr_invalid_response"):
		return "rapidocr_ocr_invalid_response"
	case strings.Contains(msg, "rapidocr_ocr_unauthorized"):
		return "rapidocr_ocr_unauthorized"
	case strings.Contains(msg, "rapidocr_ocr_forbidden"):
		return "rapidocr_ocr_forbidden"
	case strings.Contains(msg, "rapidocr_ocr_http_"):
		return "rapidocr_ocr_http_error"
	case strings.Contains(msg, "rapidocr_ocr_unavailable"):
		return "rapidocr_ocr_unavailable"
	case strings.Contains(msg, "llm_ocr_disabled"):
		return "llm_ocr_disabled"
	case strings.Contains(msg, "llm_ocr_failed"):
		return "llm_ocr_failed"
	case strings.Contains(msg, "llm_ocr_empty_content"):
		return "llm_ocr_empty_content"
	case strings.Contains(msg, "llm_ocr_unprocessable"):
		return "llm_ocr_unprocessable"
	case strings.Contains(msg, "llm_ocr_invalid_response"):
		return "llm_ocr_invalid_response"
	case strings.Contains(msg, "llm_ocr_unauthorized"):
		return "llm_ocr_unauthorized"
	case strings.Contains(msg, "llm_ocr_forbidden"):
		return "llm_ocr_forbidden"
	case strings.Contains(msg, "llm_ocr_http_"):
		return "llm_ocr_http_error"
	case strings.Contains(msg, "llm_ocr_unavailable"):
		return "llm_ocr_unavailable"
	case strings.Contains(msg, "ocr_disabled"):
		return "ocr_disabled"
	case strings.Contains(msg, "ocr_failed"):
		return "ocr_failed"
	case strings.Contains(msg, "ocr_empty_content"):
		return "ocr_empty_content"
	case strings.Contains(msg, "ocr_unprocessable"):
		return "ocr_unprocessable"
	case strings.Contains(msg, "ocr_unauthorized"):
		return "ocr_unauthorized"
	case strings.Contains(msg, "ocr_forbidden"):
		return "ocr_forbidden"
	case strings.Contains(msg, "ocr_http_"):
		return "ocr_http_error"
	case strings.Contains(msg, "tika_empty_content"):
		return "tika_empty_content"
	case strings.Contains(msg, "tika_unprocessable"):
		return "tika_unprocessable"
	case strings.Contains(msg, "tika_unauthorized"):
		return "tika_unauthorized"
	case strings.Contains(msg, "tika_forbidden"):
		return "tika_forbidden"
	case strings.Contains(msg, "tika_unsupported_media_type"):
		return "tika_unsupported_media_type"
	case strings.Contains(msg, "tika_http_"):
		return "tika_http_error"
	case strings.Contains(msg, "deadline") || strings.Contains(msg, "timeout"):
		return "extract_timeout"
	default:
		return "extract_failed"
	}
}

func resolveProcessingFailure(fileObj *domainconversation.FileObject, err error) (string, string) {
	code := classifyProcessingErrorCode(err)
	category := ""
	if fileObj != nil {
		category = fileObj.FileCategory
	}
	return code, HumanizeFileProcessingError(category, code, "")
}

func processingErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	code := classifyProcessingErrorCode(err)
	return HumanizeFileProcessingError("", code, "")
}

func resolveProcessingFailureMessage(fileObj *domainconversation.FileObject, code string, err error) string {
	if err == nil {
		return ""
	}
	category := ""
	if fileObj != nil {
		category = fileObj.FileCategory
	}
	return HumanizeFileProcessingError(category, code, err.Error())
}

func HumanizeFileProcessingError(fileCategory string, code string, message string) string {
	raw := strings.TrimSpace(message)
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	if raw == "" {
		raw = normalizedCode
	}
	lower := strings.ToLower(raw)

	switch normalizedCode {
	case "extract_timeout":
		return "文件提取超时，请稍后重试，或缩小文件后重试。"
	case "tesseract_ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 Tesseract OCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "tesseract_ocr_failed":
		return "PDF 未提取到可读文本，且 Tesseract OCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "tesseract_ocr_empty_content":
		return "Tesseract OCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "tesseract_ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "tesseract_ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "Tesseract OCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "Tesseract OCR 服务无法处理该 PDF: " + detail
	case "tesseract_ocr_invalid_response":
		return "Tesseract OCR 服务返回格式不符合当前页级协议，无法合并识别结果。"
	case "tesseract_ocr_unauthorized":
		return "Tesseract OCR 服务鉴权失败，请检查鉴权密钥配置。"
	case "tesseract_ocr_forbidden":
		return "Tesseract OCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "tesseract_ocr_http_error":
		if strings.HasPrefix(lower, "tesseract_ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "Tesseract OCR 服务请求失败: " + codePart
				}
				return "Tesseract OCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "Tesseract OCR 服务请求失败: " + raw
		}
		return "Tesseract OCR 服务请求失败。"
	case "tesseract_ocr_unavailable":
		return "当前 Tesseract OCR 服务不可用，无法从扫描件中提取文本。"
	case "rapidocr_ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 RapidOCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "rapidocr_ocr_failed":
		return "PDF 未提取到可读文本，且 RapidOCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "rapidocr_ocr_empty_content":
		return "RapidOCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "rapidocr_ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "rapidocr_ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "RapidOCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "RapidOCR 服务无法处理该 PDF: " + detail
	case "rapidocr_ocr_invalid_response":
		return "RapidOCR 服务返回格式不符合当前页级协议，无法合并识别结果。"
	case "rapidocr_ocr_unauthorized":
		return "RapidOCR 服务鉴权失败，请检查鉴权密钥配置。"
	case "rapidocr_ocr_forbidden":
		return "RapidOCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "rapidocr_ocr_http_error":
		if strings.HasPrefix(lower, "rapidocr_ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "RapidOCR 服务请求失败: " + codePart
				}
				return "RapidOCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "RapidOCR 服务请求失败: " + raw
		}
		return "RapidOCR 服务请求失败。"
	case "rapidocr_ocr_unavailable":
		return "当前 RapidOCR 服务不可用，无法从扫描件中提取文本。"
	case "llm_ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 LLM OCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "llm_ocr_failed":
		return "PDF 未提取到可读文本，且 LLM OCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "llm_ocr_empty_content":
		return "LLM OCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "llm_ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "llm_ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "LLM OCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "LLM OCR 服务无法处理该 PDF: " + detail
	case "llm_ocr_invalid_response":
		return "LLM OCR 服务返回格式不符合当前页级协议，无法合并识别结果。"
	case "llm_ocr_unauthorized":
		return "LLM OCR 服务鉴权失败，请检查鉴权密钥配置。"
	case "llm_ocr_forbidden":
		return "LLM OCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "llm_ocr_http_error":
		if strings.HasPrefix(lower, "llm_ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "LLM OCR 服务请求失败: " + codePart
				}
				return "LLM OCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "LLM OCR 服务请求失败: " + raw
		}
		return "LLM OCR 服务请求失败。"
	case "llm_ocr_unavailable":
		return "当前 LLM OCR 服务不可用，无法从扫描件中提取文本。"
	case "ocr_disabled":
		return "PDF 未提取到可读文本，且当前未启用 OCR。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "ocr_failed":
		return "PDF 未提取到可读文本，且 OCR 识别失败。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
	case "ocr_empty_content":
		return "OCR 未识别出可读文本。该 PDF 可能是空白页、图片质量过低，或内容本身不可识别。"
	case "ocr_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "ocr_unprocessable:"))
		if detail == "" || detail == raw {
			return "OCR 服务无法处理该 PDF。文件可能已损坏、加密，或超出当前 OCR 服务能力。"
		}
		return "OCR 服务无法处理该 PDF: " + detail
	case "ocr_unauthorized":
		return "OCR 服务鉴权失败，请检查 OCR 鉴权密钥配置。"
	case "ocr_forbidden":
		return "OCR 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "ocr_http_error":
		if strings.HasPrefix(lower, "ocr_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "OCR 服务请求失败: " + codePart
				}
				return "OCR 服务请求失败: " + codePart + " - " + msgPart
			}
			return "OCR 服务请求失败: " + raw
		}
		return "OCR 服务请求失败。"
	case "tika_empty_content":
		if strings.ToLower(strings.TrimSpace(fileCategory)) == "pdf" {
			return "Tika 未提取到可读文本。该 PDF 可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"
		}
		return "Tika 未提取到可读文本。文件内容可能主要为图片、空白页，或文档本身不含可复制文本。"
	case "tika_unprocessable":
		detail := strings.TrimSpace(strings.TrimPrefix(raw, "tika_unprocessable:"))
		if detail == "" || detail == raw {
			return "Tika 无法处理该文件。文件可能已损坏、加密，或格式超出当前解析能力。"
		}
		return "Tika 无法处理该文件: " + detail
	case "tika_unauthorized":
		return "Tika 服务鉴权失败，请检查 Tika Token 配置。"
	case "tika_forbidden":
		return "Tika 服务拒绝访问，请检查服务端鉴权或访问控制配置。"
	case "tika_unsupported_media_type":
		return "Tika 不支持当前文件类型或 MIME 类型。"
	case "tika_http_error":
		detail := raw
		if strings.HasPrefix(lower, "tika_http_") {
			if idx := strings.Index(raw, ":"); idx >= 0 {
				codePart := strings.TrimSpace(raw[:idx])
				msgPart := strings.TrimSpace(raw[idx+1:])
				if msgPart == "" {
					return "Tika 服务请求失败: " + codePart
				}
				return "Tika 服务请求失败: " + codePart + " - " + msgPart
			}
			return "Tika 服务请求失败: " + raw
		}
		if detail == "" {
			return "Tika 服务请求失败。"
		}
		return "Tika 服务请求失败: " + detail
	}

	if strings.ToLower(strings.TrimSpace(fileCategory)) == "pdf" {
		switch {
		case raw == "extract_failed", raw == "pdf_no_extractable_text":
			return "PDF 未提取到可读文本。该文件可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"
		case raw == "ocr_unavailable":
			return "PDF 未提取到可读文本，且当前 OCR 服务不可用。该文件可能是扫描件、图片型 PDF 或加密 PDF。"
		case strings.HasPrefix(lower, "pdf_parse_failed:"):
			detail := strings.TrimSpace(raw[len("pdf_parse_failed:"):])
			if detail == "" {
				return "PDF 解析失败，请检查文件是否损坏、加密，或格式是否异常。"
			}
			return "PDF 解析失败: " + detail
		}
	}

	switch raw {
	case "extract_failed":
		return "无法提取文本，请检查文件是否损坏、加密，或内容是否主要为图片。"
	case "ocr_unavailable":
		return "当前 OCR 服务不可用，无法从扫描件中提取文本。"
	}

	return raw
}

func compactSnippet(content string, maxLen int) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if value == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 120
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
}

func truncateError(message string, limit int) string {
	value := strings.TrimSpace(message)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func supportsExtraction(category string) bool {
	switch category {
	case "pdf", "word", "presentation", "excel", "text", "image":
		return true
	default:
		return false
	}
}

func supportsRAG(category string) bool {
	switch category {
	case "pdf", "word", "presentation", "excel", "text", "image":
		return true
	default:
		return false
	}
}
