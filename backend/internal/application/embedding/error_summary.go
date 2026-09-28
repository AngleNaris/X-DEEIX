package embedding

import (
	"context"
	"errors"
)

var (
	ErrEmbeddingServiceUnavailable   = errors.New("embedding service unavailable")
	ErrEmbeddingQueueUnavailable     = errors.New("embedding queue unavailable")
	ErrTooManyTargetedFiles          = errors.New("too many files for targeted embedding")
	errEmbeddingConfigurationChanged = errors.New("embedding configuration changed")
)

const (
	embeddingErrorLimit           = 255
	embeddingFailureMessage       = "向量化失败，请稍后重试。"
	embeddingUnavailableMessage   = "向量化服务暂时不可用，请稍后重试。"
	embeddingNotConfiguredMessage = "向量化服务尚未配置。"
	embeddingTimeoutMessage       = "向量化超时，请稍后重试。"
	embeddingCanceledMessage      = "向量化已取消。"
	embeddingNoTextMessage        = "无法读取文件提取文本。"
	embeddingEmptyChunksMessage   = "文件没有可用于向量化的内容。"
	embeddingConfigurationChanged = "向量化配置已变更，请重新提交任务。"
)

// ErrorSummary exposes stable descriptions, never raw upstream or storage errors.
func ErrorSummary(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return embeddingCanceledMessage
	case errors.Is(err, context.DeadlineExceeded):
		return embeddingTimeoutMessage
	case errors.Is(err, ErrEmbeddingServiceNotConfigured):
		return embeddingNotConfiguredMessage
	case errors.Is(err, ErrEmbeddingServiceUnavailable):
		return embeddingUnavailableMessage
	case errors.Is(err, errNoExtractableText):
		return embeddingNoTextMessage
	case errors.Is(err, errEmptyChunks):
		return embeddingEmptyChunksMessage
	case errors.Is(err, errEmbeddingConfigurationChanged):
		return embeddingConfigurationChanged
	default:
		return embeddingFailureMessage
	}
}
