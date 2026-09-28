package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const (
	resumeActiveCheckInterval         = 5 * time.Second
	usageAuthorizationRenewalInterval = 30 * time.Minute
)

var reservedMessageOptionKeys = map[string]struct{}{
	"contents":          {},
	"instructions":      {},
	"input":             {},
	"messages":          {},
	"model":             {},
	"prompt":            {},
	"stream":            {},
	"system":            {},
	"systemInstruction": {},
}

func sanitizeMessageOptions(options map[string]interface{}) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}
	sanitized := make(map[string]interface{}, len(options))
	for key, value := range options {
		if _, ok := reservedMessageOptionKeys[key]; ok {
			continue
		}
		sanitized[key] = value
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func validateAgentGroupMessageModel(conversation *model.Conversation, requestModel string) error {
	if conversation != nil &&
		!model.AgentGroupRequestModelAllowed(conversation.AgentGroupID != nil, requestModel) {
		return appconversation.ErrConversationModelNotAllowedWithGroup
	}
	return nil
}

// parseSendMessageInput 解析消息发送请求的公共参数。
func (h *Handler) parseSendMessageInput(c *gin.Context) (appconversation.SendMessageInput, *model.Conversation, *SendMessageRequest, error) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, apperr.New(response.CodeRequestInvalidID, "invalid conversation id"))
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	var req SendMessageRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return appconversation.SendMessageInput{}, nil, nil, err
	}
	req.ClientRunID = appconversation.EnsureMessageGenerationRunID(req.ClientRunID)
	req.Options = sanitizeMessageOptions(req.Options)
	// 流式接口写入响应头前先拦截明显超限请求，避免后续只能用 NDJSON error 表达 400。
	if err = h.service.ValidateSelectedToolIDs(req.SelectedToolIDs); err != nil {
		handleSendMessageError(c, err)
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrConversationNotFound)
			return appconversation.SendMessageInput{}, nil, nil, err
		}
		response.InternalError(c)
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	// 群组会话的模型由群组成员覆盖与角色默认值决定，禁止请求级模型覆盖。
	if err = validateAgentGroupMessageModel(conversation, req.Model); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, apperr.New("agent_group.model_override_not_allowed", "model override not allowed in agent group conversation"))
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	input := appconversation.SendMessageInput{
		UserID:                  userID,
		ConversationID:          conversation.ID,
		RequestID:               middleware.MustRequestID(c),
		ContentType:             req.ContentType,
		Content:                 req.Content,
		PlatformModelName:       req.Model,
		ModelScope:              req.ModelScope,
		UserModelID:             req.UserModelID,
		Options:                 req.Options,
		ClientRunID:             req.ClientRunID,
		FileIDs:                 req.FileIDs,
		SelectedToolIDs:         req.SelectedToolIDs,
		SkillIDs:                req.SkillIDs,
		KnowledgeBaseIDs:        req.KnowledgeBaseIDs,
		HTMLVisualPromptEnabled: req.HTMLVisualPromptEnabled,
		UIComponentIDs:          req.UIComponentIDs,
		ParentMessagePublicID:   req.ParentMessagePublicID,
		SourceMessagePublicID:   req.SourceMessagePublicID,
		BranchReason:            req.BranchReason,
	}

	return input, conversation, &req, nil
}

func sendMessageBillingInput(
	userID uint,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
) appconversation.SendMessageBillingInput {
	input := appconversation.SendMessageBillingInput{
		UserID:            userID,
		PlatformModelName: strings.TrimSpace(req.Model),
		ClientRunID:       strings.TrimSpace(req.ClientRunID),
		Result:            result,
	}
	if conversation != nil {
		input.ConversationID = conversation.ID
		input.ConversationModel = conversation.Model
		input.Conversation = conversation
	}
	return input
}

func (h *Handler) reserveUsage(c *gin.Context, input appconversation.SendMessageBillingInput) (*domainbilling.UsageAuthorization, error) {
	return h.service.AuthorizeSendMessageUsage(
		c.Request.Context(),
		input,
	)
}

// authorizeUsage 在写入流式响应头前完成请求级计费授权。
func (h *Handler) authorizeUsage(c *gin.Context, input appconversation.SendMessageBillingInput) (*domainbilling.UsageAuthorization, error) {
	authorization, err := h.reserveUsage(c, input)
	if err != nil {
		handleUsageAuthorizationError(c, err)
		return nil, err
	}
	return authorization, nil
}

// authorizeMessageUsage 将终态拒绝的持久化委托给应用层，再把授权结果转换为 HTTP 响应。
func (h *Handler) authorizeMessageUsage(
	c *gin.Context,
	input appconversation.SendMessageInput,
	billingInput appconversation.SendMessageBillingInput,
) (*domainbilling.UsageAuthorization, error) {
	authorization, err := h.reserveUsage(c, billingInput)
	if err == nil {
		return authorization, nil
	}
	if persistErr := h.service.PersistMessageUsageRejection(c.Request.Context(), input, err); persistErr != nil {
		handleSendMessageError(c, persistErr)
		return nil, persistErr
	}
	handleUsageAuthorizationError(c, err)
	return nil, err
}

// releaseSendMessageUsageAuthorization 使用独立短上下文释放未消费的预算。
func (h *Handler) releaseSendMessageUsageAuthorization(authorization *domainbilling.UsageAuthorization) error {
	if authorization == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.service.ReleaseSendMessageUsageAuthorization(ctx, authorization)
}

// startUsageAuthorizationRenewal 为长时间运行的调用持续刷新预算租约。
func (h *Handler) startUsageAuthorizationRenewal(authorization *domainbilling.UsageAuthorization) func() {
	if authorization == nil || authorization.Reservation == nil {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(usageAuthorizationRenewalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := h.service.RenewSendMessageUsageAuthorization(ctx, authorization)
				cancel()
				if err != nil {
					continue
				}
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

// recordAndApplySendMessageBilling 统一记录账单并把快照回填到当前响应消息，避免流式和非流式口径分叉。
func (h *Handler) recordAndApplySendMessageBilling(
	ctx context.Context,
	userID uint,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	authorization *domainbilling.UsageAuthorization,
) error {
	usageLedger, err := h.service.RecordSendMessageBilling(
		ctx,
		sendMessageBillingInput(userID, conversation, req, result),
		authorization,
	)
	if err != nil {
		return err
	}
	appconversation.ApplyUsageBilling(&result.AssistantMessage, usageLedger)
	return nil
}

// recordSendMessageAudit 记录审计日志（同步，供非流式路径使用）。
func (h *Handler) recordSendMessageAudit(c *gin.Context, conversation *model.Conversation, req *SendMessageRequest, result *appconversation.SendMessageResult, action string) {
	h.recordSendMessageAuditCtx(
		c.Request.Context(),
		middleware.MustUserID(c),
		middleware.MustRequestID(c),
		c.ClientIP(),
		c.Request.UserAgent(),
		conversation, req, result, action,
	)
}

// recordStreamSendMessageAuditAsync 在 Handler 返回前提取 gin.Context 值，goroutine 内不持有 gin.Context。
func (h *Handler) recordStreamSendMessageAuditAsync(
	c *gin.Context,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	action string,
) {
	bgUserID := middleware.MustUserID(c)
	bgRequestID := middleware.MustRequestID(c)
	bgClientIP := c.ClientIP()
	bgUserAgent := c.Request.UserAgent()
	go h.recordSendMessageAuditCtx(
		context.Background(),
		bgUserID, bgRequestID, bgClientIP, bgUserAgent,
		conversation, req, result, action,
	)
}

// recordSendMessageAuditCtx 接受显式参数，可在 goroutine 中安全调用（不依赖 gin.Context）。
func (h *Handler) recordSendMessageAuditCtx(
	ctx context.Context,
	userID uint,
	requestID string,
	clientIP string,
	userAgent string,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	action string,
) {
	h.service.RecordSendMessageAudit(
		ctx,
		appconversation.SendMessageAuditInput{
			UserID:         userID,
			RequestID:      requestID,
			ClientIP:       clientIP,
			UserAgent:      userAgent,
			Action:         action,
			ContentType:    req.ContentType,
			ConversationID: conversation.ID,
			FileIDs:        req.FileIDs,
			Result:         result,
		},
	)
}

func handleSendMessageBillingError(c *gin.Context, err error) {
	if errors.Is(err, billing.ErrUsageConcurrencyLimitExceeded) {
		response.ErrorFrom(c, http.StatusTooManyRequests, billing.ErrUsageConcurrencyLimitExceeded)
		return
	}
	if errors.Is(err, billing.ErrUsageReservationConflict) {
		response.ErrorFrom(c, http.StatusConflict, billing.ErrUsageReservationConflict)
		return
	}
	if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
		response.ErrorFrom(c, http.StatusPaymentRequired, billing.ErrUsageBalanceInsufficient)
		return
	}
	if errors.Is(err, billing.ErrModelPricingRequired) {
		response.ErrorFrom(c, http.StatusPaymentRequired, billing.ErrModelPricingRequired)
		return
	}
	response.InternalError(c)
}

func handleUsageAuthorizationError(c *gin.Context, err error) {
	if errors.Is(err, billing.ErrUsageConcurrencyLimitExceeded) {
		response.ErrorFrom(c, http.StatusTooManyRequests, billing.ErrUsageConcurrencyLimitExceeded)
		return
	}
	if errors.Is(err, billing.ErrUsageReservationConflict) {
		response.ErrorFrom(c, http.StatusConflict, billing.ErrUsageReservationConflict)
		return
	}
	if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
		response.ErrorFrom(c, http.StatusPaymentRequired, billing.ErrUsageBalanceInsufficient)
		return
	}
	if errors.Is(err, billing.ErrModelPricingRequired) {
		response.ErrorFrom(c, http.StatusPaymentRequired, billing.ErrModelPricingRequired)
		return
	}
	response.InternalError(c)
}

func mapBillingStreamError(err error) streamError {
	status := http.StatusInternalServerError
	message := "record billing failed"
	if errors.Is(err, billing.ErrUsageConcurrencyLimitExceeded) {
		status = http.StatusTooManyRequests
		message = "usage concurrency limit exceeded"
	}
	if errors.Is(err, billing.ErrUsageReservationConflict) {
		status = http.StatusConflict
		message = "usage reservation already exists"
	}
	if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
		status = http.StatusPaymentRequired
		message = "usage balance is insufficient"
	}
	if errors.Is(err, billing.ErrModelPricingRequired) {
		status = http.StatusPaymentRequired
		message = "model pricing is required"
	}
	code := response.InferErrorCode(status, message)
	return streamError{
		Status:  status,
		Code:    code,
		Message: response.PublicErrorMessage(status, code, message),
	}
}

func billingStreamErrorPayload(err error) map[string]interface{} {
	mapped := mapBillingStreamError(err)
	return map[string]interface{}{
		"type":      "error",
		"message":   mapped.Message,
		"errorCode": mapped.Code,
	}
}

// handleSendMessageError 处理发送消息错误的公共方法。
func handleSendMessageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconversation.ErrConversationNotFound):
		response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrConversationNotFound)
	case errors.Is(err, appconversation.ErrInvalidFileReference):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrInvalidFileReference)
	case errors.Is(err, appconversation.ErrFileNotFound):
		response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
	case errors.Is(err, appconversation.ErrFileTooLarge):
		response.ErrorFrom(c, http.StatusRequestEntityTooLarge, appconversation.ErrFileTooLarge)
	case errors.Is(err, appconversation.ErrTooManyMessageFiles):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrTooManyMessageFiles)
	case errors.Is(err, appconversation.ErrTooManySelectedTools):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrTooManySelectedTools)
	case errors.Is(err, appconversation.ErrMultipleImageAttachmentProcessors):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrMultipleImageAttachmentProcessors)
	case errors.Is(err, appconversation.ErrImageAttachmentProcessingFailed):
		response.ErrorFrom(c, http.StatusBadGateway, appconversation.ErrImageAttachmentProcessingFailed)
	case errors.Is(err, appconversation.ErrTooManySelectedSkills):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrTooManySelectedSkills)
	case errors.Is(err, appconversation.ErrSkillNotFound):
		response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrSkillNotFound)
	case errors.Is(err, appconversation.ErrInvalidSkillUse):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrInvalidSkillUse)
	case errors.Is(err, appconversation.ErrInvalidMessageBranch):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrInvalidMessageBranch)
	case errors.Is(err, appconversation.ErrFileProcessingNotReady):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrFileProcessingNotReady)
	case errors.Is(err, appconversation.ErrFileTooLargeForFullContext):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrFileTooLargeForFullContext)
	case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
		response.ErrorFrom(c, http.StatusBadRequest, appconversation.ErrEmbeddingUnavailable)
	case errors.Is(err, appconversation.ErrInvalidKnowledgeBaseReference):
		response.ErrorFrom(c, http.StatusBadRequest, apperr.New(appconversation.MessageErrorCodeKnowledgeBaseInvalidReference, "invalid knowledge base reference"))
	case errors.Is(err, appconversation.ErrKnowledgeBaseUnavailable):
		response.ErrorFrom(c, http.StatusServiceUnavailable, apperr.New(appconversation.MessageErrorCodeKnowledgeBaseUnavailable, "knowledge base retrieval is unavailable"))
	case errors.Is(err, appconversation.ErrKnowledgeBaseNotReady):
		response.ErrorFrom(c, http.StatusConflict, apperr.New(appconversation.MessageErrorCodeKnowledgeBaseNotReady, "selected knowledge base has no ready files"))
	case errors.Is(err, appconversation.ErrModelRouteNotConfigured):
		response.ErrorFrom(c, http.StatusServiceUnavailable, appconversation.ErrModelRouteNotConfigured)
	case errors.Is(err, appconversation.ErrGeneratedMediaArtifactUnavailable):
		response.ErrorFrom(c, http.StatusBadGateway, apperr.New(appconversation.MessageErrorCode(err), "generated media artifact is temporarily unavailable"))
	case errors.Is(err, appconversation.ErrUpstreamEmptyResponse):
		response.ErrorFrom(c, http.StatusBadGateway, appconversation.ErrUpstreamEmptyResponse)
	case errors.Is(err, appconversation.ErrUpstreamRequestFailed):
		if code := appconversation.MessageErrorCode(err); code != "" {
			response.ErrorFrom(c, http.StatusBadGateway, apperr.New(code, mapClientErrorMessage(err)))
			return
		}
		response.ErrorFrom(c, http.StatusBadGateway, appconversation.ErrUpstreamRequestFailed)
	case errors.Is(err, appconversation.ErrAgentGroupFeatureDisabled):
		response.ErrorFrom(c, http.StatusForbidden, apperr.New("agent_group.feature_disabled", "agent group feature disabled"))
	case errors.Is(err, appconversation.ErrAgentGroupRunInProgress):
		response.ErrorFrom(c, http.StatusConflict, apperr.New("agent_group.run_in_progress", "agent group run already in progress"))
	case errors.Is(err, appconversation.ErrAgentGroupRunPaused):
		response.ErrorFrom(c, http.StatusConflict, apperr.New("agent_group.run_paused", "agent group run paused"))
	case errors.Is(err, appconversation.ErrAgentGroupRunBlocked):
		response.ErrorFrom(c, http.StatusConflict, apperr.New("agent_group.run_blocked", "agent group run blocked"))
	case errors.Is(err, appconversation.ErrAgentGroupCASConflict):
		response.ErrorFrom(c, http.StatusConflict, apperr.New("agent_group.run_state_conflict", "agent group run state conflict"))
	default:
		response.InternalError(c)
	}
}

// SendMessage godoc
// @Summary 发送消息
// @Description 在会话中发送消息，支持文件/图片等多模态附件
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body SendMessageRequest true "消息参数"
// @Success 200 {object} SendMessageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages [post]
// SendMessage 发送消息。
func (h *Handler) SendMessage(c *gin.Context) {
	input, conversation, req, err := h.parseSendMessageInput(c)
	if err != nil {
		return
	}
	authorization, err := h.authorizeMessageUsage(c, input, sendMessageBillingInput(middleware.MustUserID(c), conversation, req, nil))
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

	result, err := h.service.SendMessage(c.Request.Context(), input)
	if err != nil {
		if result != nil {
			if !result.Billable {
				if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
					handleSendMessageBillingError(c, releaseErr)
					return
				}
				handleSendMessageError(c, err)
				return
			}
			if billingErr := h.recordAndApplySendMessageBilling(c.Request.Context(), middleware.MustUserID(c), conversation, req, result, authorization); billingErr != nil {
				handleSendMessageBillingError(c, billingErr)
				return
			}
			h.recordSendMessageAudit(c, conversation, req, result, "send_message")
			response.Success(c, toSendMessageResponse(result))
			return
		}
		if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
			handleSendMessageBillingError(c, releaseErr)
			return
		}
		handleSendMessageError(c, err)
		return
	}

	// Agent 群组运行的计费发生在各内部 Attempt（逐次记入 BillingRef），顶层仅释放用量授权。
	if !result.Billable {
		if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
			handleSendMessageBillingError(c, releaseErr)
			return
		}
		h.recordSendMessageAudit(c, conversation, req, result, "send_message")
		response.Success(c, toSendMessageResponse(result))
		return
	}
	if err := h.recordAndApplySendMessageBilling(c.Request.Context(), middleware.MustUserID(c), conversation, req, result, authorization); err != nil {
		handleSendMessageBillingError(c, err)
		return
	}
	h.recordSendMessageAudit(c, conversation, req, result, "send_message")
	response.Success(c, toSendMessageResponse(result))
}

// StreamMessage godoc
// @Summary 流式发送消息
// @Description 在会话中发送消息并以 NDJSON 流式返回 assistant 增量文本
// @Tags chat
// @Accept json
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body SendMessageRequest true "消息参数"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages/stream [post]
func (h *Handler) StreamMessage(c *gin.Context) {
	generationCtx, releaseLifecycle, ok := h.service.AcquireMessageGenerationLifecycle(context.WithoutCancel(c.Request.Context()))
	if !ok {
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
		return
	}
	defer releaseLifecycle()
	input, conversation, req, err := h.parseSendMessageInput(c)
	if err != nil {
		return
	}
	authorization, err := h.authorizeMessageUsage(c, input, sendMessageBillingInput(middleware.MustUserID(c), conversation, req, nil))
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var clientDisconnected atomic.Bool
	flushStreamEvent := func(payload map[string]interface{}) error {
		var owned bool
		payload, owned = h.service.PublishMessageGenerationEvent(generationCtx, input.ClientRunID, payload)
		if !owned {
			return nil
		}
		if clientDisconnected.Load() {
			return nil
		}
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := c.Writer.Write(append(encoded, '\n')); writeErr != nil {
			clientDisconnected.Store(true)
			return writeErr
		}
		c.Writer.Flush()
		return nil
	}

	// 有附件时先推送文件处理事件，提升用户体验感知。
	if len(req.FileIDs) > 0 {
		_ = flushStreamEvent(map[string]interface{}{
			"type":    "file_proc",
			"message": "正在处理附件…",
		})
	}

	// 将中间事件（含 moderation_*）通过 NDJSON 推送给客户端。
	input.OnEvent = func(eventType string, payload map[string]interface{}) error {
		_ = flushStreamEvent(normalizeStreamEventPayload(eventType, payload))
		return nil
	}

	defer h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
	result, err := h.service.StreamMessage(generationCtx, input, func(delta string) error {
		_ = flushStreamEvent(map[string]interface{}{
			"type":  "delta",
			"delta": delta,
		})
		return nil
	})
	if err == nil && result != nil && result.IsModerationBlocked() {
		// Guarantee a terminal event even if live OnEvent path missed emit.
		if !result.ModerationTerminalEmitted() {
			_ = flushStreamEvent(moderationBlockedStreamPayload(result))
		}
		if result.Billable {
			billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, authorization)
			billingCancel()
		} else {
			_ = h.releaseSendMessageUsageAuthorization(authorization)
		}
		h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
		h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
		return
	}
	if err != nil {
		if result != nil {
			if !result.Billable {
				if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
					_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
					h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
					return
				}
				payload := streamErrorPayload(err)
				payload["data"] = toSendMessageResponse(result)
				if debug := appconversation.MessageErrorDebug(err); debug != nil {
					payload["debug"] = debug
				}
				_ = flushStreamEvent(payload)
				h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
				h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
				return
			}
			billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
			billingErr := h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, authorization)
			billingCancel()
			if billingErr != nil {
				payload := billingStreamErrorPayload(billingErr)
				payload["data"] = toSendMessageResponse(result)
				_ = flushStreamEvent(payload)
				h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
				return
			}
			payload := streamErrorPayload(err)
			payload["data"] = toSendMessageResponse(result)
			if debug := appconversation.MessageErrorDebug(err); debug != nil {
				payload["debug"] = debug
			}
			_ = flushStreamEvent(payload)
			h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
			h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
			return
		}
		if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
			_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
			h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
			return
		}
		payload := streamErrorPayload(err)
		if debug := appconversation.MessageErrorDebug(err); debug != nil {
			payload["debug"] = debug
		}
		_ = flushStreamEvent(payload)
		h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
		return
	}

	// Agent 群组运行的计费发生在各内部 Attempt，顶层仅释放用量授权。
	if !result.Billable {
		if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
			_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
			h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
			return
		}
		_ = flushStreamEvent(map[string]interface{}{
			"type": "completed",
			"data": toSendMessageResponse(result),
		})
		h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
		h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
		return
	}

	billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	billingErr := h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, authorization)
	billingCancel()
	if billingErr != nil {
		_ = flushStreamEvent(billingStreamErrorPayload(billingErr))
		h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
		return
	}

	_ = flushStreamEvent(map[string]interface{}{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
	h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
	h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
}

// CancelMessageGeneration godoc
// @Summary 取消流式生成
// @Description 仅在用户显式点击暂停时取消对应 run；浏览器刷新或断开连接不会调用此接口
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/cancel [post]
func (h *Handler) CancelMessageGeneration(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, apperr.New(response.CodeRequestInvalidID, "invalid run id"))
		return
	}
	canceled := h.service.CancelMessageGeneration(c.Request.Context(), middleware.MustUserID(c), runID)
	response.Success(c, CancelMessageGenerationResponse{Canceled: canceled})
}

// RequeryMediaVideoRun godoc
// @Summary 重查失败的视频生成任务
// @Description 按运行记录中的上游任务 ID 回原上游查询一次：completed 时回收产物并补写消息附件，返回最新状态
// @Tags chat
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Success 200 {object} RequeryMediaVideoRunResponse
// @Failure 400 {object} response.Envelope
// @Failure 404 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/media/requery [post]
func (h *Handler) RequeryMediaVideoRun(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidRunID)
		return
	}
	result, err := h.service.RequeryMediaVideoRun(c.Request.Context(), middleware.MustUserID(c), runID)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	payload := RequeryMediaVideoRunResponse{
		Status: string(result.Status),
		RunID:  result.RunID,
	}
	if result.Message != "" {
		msg := result.Message
		payload.Message = &msg
	}
	for _, item := range result.Attachments {
		payload.Attachments = append(payload.Attachments, RequeryMediaVideoAttachmentResponse{
			FileID:          item.FileID,
			FileName:        item.FileName,
			MimeType:        item.MimeType,
			SizeBytes:       item.SizeBytes,
			DurationSeconds: item.DurationSeconds,
		})
	}
	response.Success(c, payload)
}

// RetryMediaImageArtifact godoc
// @Summary 重试保存待保存的图像产物
// @Description 上游生成已成功但产物下载瞬时失败时，输出节点按 runID + 产物序号重试一次保存：重新下载 -> 上传 -> 追加消息附件
// @Tags chat
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Accept json
// @Param body body object true "产物序号 {index}"
// @Success 200 {object} RetryMediaImageArtifactResponse
// @Failure 400 {object} response.Envelope
// @Failure 404 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/media/artifact-retry [post]
func (h *Handler) RetryMediaImageArtifact(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidRunID)
		return
	}
	var req struct {
		Index *int `json:"index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Index == nil || *req.Index < 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidRunID)
		return
	}
	result, err := h.service.RetryMediaImageArtifact(c.Request.Context(), middleware.MustUserID(c), runID, *req.Index)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	payload := RetryMediaImageArtifactResponse{
		Status: string(result.Status),
		RunID:  result.RunID,
		Index:  result.Index,
	}
	if result.Message != "" {
		msg := result.Message
		payload.Message = &msg
	}
	if result.Attachment != nil {
		payload.Attachment = &RetryMediaImageArtifactAttachmentResult{
			FileID:    result.Attachment.FileID,
			FileName:  result.Attachment.FileName,
			MimeType:  result.Attachment.MimeType,
			SizeBytes: result.Attachment.SizeBytes,
		}
	}
	response.Success(c, payload)
}

// ResumeMessageGenerationStream godoc
// @Summary 恢复流式生成订阅
// @Description 页面刷新后按 run_id 重新订阅仍在运行的生成流，返回 NDJSON 事件
// @Tags chat
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Param after query int false "已接收的最后事件序号"
// @Param snapshot query bool false "是否返回可替换当前正文的权威文本快照"
// @Success 200 {string} string "NDJSON stream"
// @Failure 404 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/stream [get]
func (h *Handler) ResumeMessageGenerationStream(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, apperr.New(response.CodeRequestInvalidID, "invalid run id"))
		return
	}
	afterSeq, _ := strconv.ParseInt(strings.TrimSpace(c.Query("after")), 10, 64)
	if afterSeq < 0 {
		afterSeq = 0
	}
	userID := middleware.MustUserID(c)
	includeTextSnapshot, _ := strconv.ParseBool(strings.TrimSpace(c.Query("snapshot")))
	replay, events, unsubscribe, ok := h.service.SubscribeMessageGeneration(
		c.Request.Context(),
		userID,
		runID,
		afterSeq,
		includeTextSnapshot,
	)
	if !ok {
		h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
		response.ErrorFrom(c, http.StatusNotFound, apperr.New(response.CodeResourceNotFound, "generation stream not found"))
		return
	}
	defer unsubscribe()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	isTerminal := func(payload map[string]interface{}) bool {
		eventType, _ := payload["type"].(string)
		return eventType == "completed" || eventType == "error" || eventType == "moderation_blocked"
	}
	terminalWritten := false
	writeEvent := func(payload map[string]interface{}) bool {
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return true
		}
		if _, writeErr := c.Writer.Write(append(encoded, '\n')); writeErr != nil {
			return false
		}
		c.Writer.Flush()
		if isTerminal(payload) {
			terminalWritten = true
		}
		return true
	}

	for _, event := range replay {
		if !writeEvent(event.Payload) {
			return
		}
	}
	if terminalWritten {
		return
	}

	isActive := func() bool {
		return h.service.HasActiveMessageGeneration(c.Request.Context(), runID)
	}
	if !isActive() {
		h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
		_ = writeEvent(streamErrorPayloadWithCode("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"))
		return
	}
	activeTicker := time.NewTicker(resumeActiveCheckInterval)
	defer func() {
		activeTicker.Stop()
	}()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-activeTicker.C:
			if !isActive() {
				h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
				_ = writeEvent(streamErrorPayloadWithCode("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"))
				return
			}
		case event, ok := <-events:
			if !ok {
				if !terminalWritten && !isActive() {
					h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
					_ = writeEvent(streamErrorPayloadWithCode("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"))
				}
				return
			}
			if !writeEvent(event.Payload) {
				return
			}
			if terminalWritten {
				return
			}
		}
	}
}
