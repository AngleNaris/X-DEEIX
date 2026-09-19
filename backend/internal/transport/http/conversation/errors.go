package conversation

import "errors"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errAtLeastOneOfFileNameOrRAGOptOutRequired = errors.New("at least one of file_name or rag_opt_out is required")
	errConversationNoTitleableContent          = errors.New("conversation has no titleable content")
	errConversationProjectLimitExceeded        = errors.New("conversation project limit exceeded")
	errFileExtractNotReady                     = errors.New("file extract is not ready")
	errFileEmbeddingUnavailable                = errors.New("embedding is unavailable for this file size")
	errFileRequired                            = errors.New("file is required")
	errGenerationStreamNotFound                = errors.New("generation stream not found")
	errInvalidBeforeMessageID                  = errors.New("invalid before message id")
	errInvalidContextArtifactID                = errors.New("invalid context artifact id")
	errInvalidConversationID                   = errors.New("invalid conversation id")
	errInvalidConversationProjectID            = errors.New("invalid conversation project id")
	errInvalidFileID                           = errors.New("invalid file id")
	errInvalidFile                             = errors.New("invalid file")
	errInvalidFileReference                    = errors.New("invalid file reference")
	errInvalidFileStream                       = errors.New("invalid file stream")
	errInvalidFileSignature                    = errors.New("invalid or expired file signature")
	errInvalidMessageID                        = errors.New("invalid message id")
	errInvalidRunID                            = errors.New("invalid run id")
	errInvalidRunIDs                           = errors.New("invalid run ids")
	errInvalidShareID                          = errors.New("invalid share id")
	errInvalidTemporaryChatMessages            = errors.New("invalid temporary chat messages")
	errInvalidToolCallID                       = errors.New("invalid tool call id")
	errLabelsRequired                          = errors.New("labels are required")
	errSharedFileNotFound                      = errors.New("shared file not found")
	errStorageQuotaExceeded                    = errors.New("storage quota exceeded")
	errTemporaryChatContextTooLarge            = errors.New("temporary chat context is too large")
	errTooManyFilesInOneMessage                = errors.New("too many files in one message")
)
