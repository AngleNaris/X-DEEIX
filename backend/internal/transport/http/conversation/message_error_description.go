package conversation

import (
	"errors"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"net/http"
)

type sendMessageErrorStatus struct {
	err    error
	status int
}

var sendMessageErrorStatuses = []sendMessageErrorStatus{
	{err: appconversation.ErrConversationNotFound, status: http.StatusNotFound},
	{err: appconversation.ErrInvalidFileReference, status: http.StatusBadRequest},
	{err: appconversation.ErrFileNotFound, status: http.StatusNotFound},
	{err: appconversation.ErrFileTooLarge, status: http.StatusRequestEntityTooLarge},
	{err: appconversation.ErrInvalidMessageBranch, status: http.StatusBadRequest},
	{err: appconversation.ErrTooManyMessageFiles, status: http.StatusBadRequest},
	{err: appconversation.ErrTooManySelectedTools, status: http.StatusBadRequest},
	{err: appconversation.ErrMultipleImageAttachmentProcessors, status: http.StatusBadRequest},
	{err: appconversation.ErrImageAttachmentProcessingFailed, status: http.StatusBadGateway},
	{err: appconversation.ErrTooManySelectedSkills, status: http.StatusBadRequest},
	{err: appconversation.ErrSkillNotFound, status: http.StatusNotFound},
	{err: appconversation.ErrInvalidSkillUse, status: http.StatusBadRequest},
	{err: appconversation.ErrFileProcessingNotReady, status: http.StatusBadRequest},
	{err: appconversation.ErrFileTooLargeForFullContext, status: http.StatusBadRequest},
	{err: appconversation.ErrEmbeddingUnavailable, status: http.StatusBadRequest},
	{err: appconversation.ErrInvalidKnowledgeBaseReference, status: http.StatusBadRequest},
	{err: appconversation.ErrKnowledgeBaseUnavailable, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrKnowledgeBaseNotReady, status: http.StatusConflict},
	{err: appconversation.ErrModelRouteNotConfigured, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrModelAccessDenied, status: http.StatusForbidden},
	{err: appconversation.ErrStorageQuotaExceeded, status: http.StatusConflict},
	{err: appconversation.ErrGeneratedMediaArtifactUnavailable, status: http.StatusBadGateway},
	{err: appconversation.ErrUpstreamEmptyResponse, status: http.StatusBadGateway},
	{err: appconversation.ErrToolRunFinalAnswerMissing, status: http.StatusBadGateway},
	{err: appconversation.ErrMessageGenerationCanceled, status: http.StatusBadRequest},
	{err: appconversation.ErrMessageGenerationInterrupted, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrMediaImagePromptRequired, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageGenerationRejectsInputs, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageEditInputRequired, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageEditTooManyInputs, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageEditInputInvalid, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaVideoPromptRequired, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaVideoInputInvalid, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaVideoTooManyInputs, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaRouteProtocolMismatch, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrInvalidMediaGenerationTask, status: http.StatusBadRequest},
	{err: appconversation.ErrDuplicateMessageGenerationRun, status: http.StatusConflict},
	{err: billing.ErrUsageConcurrencyLimitExceeded, status: http.StatusTooManyRequests},
	{err: billing.ErrUsageReservationConflict, status: http.StatusConflict},
	{err: billing.ErrUsageBalanceInsufficient, status: http.StatusPaymentRequired},
	{err: billing.ErrModelPricingRequired, status: http.StatusPaymentRequired},
}

func describeSendMessageError(err error) response.Description {
	// Preserve the shared repository sentinel identity while selecting the public contract.
	if errors.Is(err, appconversation.ErrStorageQuotaExceeded) {
		return response.Describe(http.StatusConflict, apperr.New("quota.exceeded", "quota exceeded"))
	}
	switch {
	case appconversation.IsUpstreamRateLimitError(err):
		return response.DescribeCode(http.StatusTooManyRequests, appconversation.MessageErrorCodeUpstreamRateLimited)
	case errors.Is(err, appconversation.ErrUpstreamRequestFailed):
		return describeUpstreamRequestFailure(err)
	}
	for _, entry := range sendMessageErrorStatuses {
		if errors.Is(err, entry.err) {
			return response.Describe(entry.status, entry.err)
		}
	}
	return response.DescribeCode(http.StatusInternalServerError, response.CodeInternal)
}

// describeUpstreamRequestFailure 描述上游请求失败。只有 application 层明确识别的上游场景
// 才使用专用错误码，其余统一返回上游不可用，避免把上游原始文案当作错误码契约。
func describeUpstreamRequestFailure(err error) response.Description {
	code := appconversation.MessageErrorCode(err)
	if code == "" {
		code = response.CodeUpstreamUnavailable
	}
	return response.DescribeCode(http.StatusBadGateway, code)
}
