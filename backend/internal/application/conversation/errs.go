package conversation

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var (
	ErrProjectFileNotFound = repository.ErrNotFound
	// ErrConversationNotFound 会话不存在或无权限。
	ErrConversationNotFound = apperr.New("conversation.not_found", "conversation not found")
	// ErrConversationEventNotFound 对话事件日志不存在。
	ErrConversationEventNotFound = apperr.New("conversation_event.not_found", "conversation event not found")
	// ErrToolCallNotFound 工具调用不存在或当前用户无权访问。
	ErrToolCallNotFound = apperr.New("tool_call.not_found", "tool call not found")
	// ErrConversationShareNotFound 会话分享不存在、已关闭或原会话已删除。
	ErrConversationShareNotFound = apperr.New("conversation_share.not_found", "conversation share not found")
	// ErrInvalidConversationShare 会话分享请求不合法。
	ErrInvalidConversationShare = apperr.New("conversation_share.invalid", "invalid conversation share")
	// ErrConversationShareSchemaOutdated 会话分享表结构未更新。
	ErrConversationShareSchemaOutdated = apperr.NewMasked("conversation_share.schema_outdated", "conversation share schema is outdated", "conversation share schema outdated")
	// ErrInvalidConversationTitle 会话标题不合法。
	ErrInvalidConversationTitle = apperr.New("conversation.invalid_title", "invalid conversation title")
	// ErrInvalidConversationLabels 会话标签不合法。
	ErrInvalidConversationLabels = apperr.New("request.invalid_conversation_labels", "invalid conversation labels")
	// ErrConversationProjectNotFound 会话项目不存在或无权限。
	ErrConversationProjectNotFound = apperr.New("conversation_project.not_found", "conversation project not found")
	// ErrConversationRoleNotFound 会话角色不存在或无权限。
	ErrConversationRoleNotFound = errors.New("conversation role not found")
	// ErrConversationAgentGroupNotFound 会话绑定的群组不存在或无权限。
	ErrConversationAgentGroupNotFound = errors.New("conversation agent group not found")
	// ErrConversationRoleInUseByAgentGroup 角色仍被未移除的群组成员引用，禁止删除（§18 删除保护）。
	ErrConversationRoleInUseByAgentGroup = errors.New("conversation role is in use by agent group member")
	// ErrConversationProjectInUseByAgentGroup 项目下仍存在群组，禁止删除（§18 删除保护）。
	ErrConversationProjectInUseByAgentGroup = errors.New("conversation project is in use by agent group")
	// ErrConversationModelNotAllowedWithGroup 群组会话禁止请求级模型覆盖。
	ErrConversationModelNotAllowedWithGroup = errors.New("conversation model override not allowed with agent group")
	// ErrConversationGroupImmutable 群组会话的绑定不可变更。
	ErrConversationGroupImmutable = errors.New("conversation agent group binding is immutable")
	// ErrAgentGroupFeatureDisabled 群组功能未启用。
	ErrAgentGroupFeatureDisabled = errors.New("agent group feature disabled")
	// ErrAgentGroupRunInProgress 会话已有进行中的群组运行。
	ErrAgentGroupRunInProgress = errors.New("agent group run already in progress")
	// ErrAgentGroupRunNotFound 群组运行不存在或无权限。
	ErrAgentGroupRunNotFound = errors.New("agent group run not found")
	// ErrAgentGroupRunNotRetryable 当前运行状态不允许重试。
	ErrAgentGroupRunNotRetryable = errors.New("agent group run not retryable")
	// ErrAgentGroupRetryRequestIDRequired 重试请求缺少客户端幂等键。
	ErrAgentGroupRetryRequestIDRequired = errors.New("agent group retry request id required")
	// ErrAgentGroupRunNotCancelable 当前运行状态不允许取消。
	ErrAgentGroupRunNotCancelable = errors.New("agent group run not cancelable")
	// ErrAgentGroupRunNotAbandonable 当前运行状态不允许放弃。
	ErrAgentGroupRunNotAbandonable = errors.New("agent group run not abandonable")
	// ErrAgentGroupRunStateCorrupt 运行持久化状态损坏，无法安全恢复执行。
	ErrAgentGroupRunStateCorrupt = errors.New("agent group run state corrupt")
	// ErrAgentGroupRunPaused 群组运行已暂停（可重试），等待重试或放弃。
	ErrAgentGroupRunPaused = errors.New("agent group run paused")
	// ErrAgentGroupRunBlocked 群组运行被阻塞，无法继续执行。
	ErrAgentGroupRunBlocked = errors.New("agent group run blocked")
	// ErrAgentGroupCASConflict 群组运行状态并发冲突。
	ErrAgentGroupCASConflict = errors.New("agent group run state conflict")
	// ErrAgentGroupInvalidDecision 主管决策无法解析或不符合协议。
	ErrAgentGroupInvalidDecision = errors.New("invalid supervisor decision")
	// ErrAgentGroupInvalidMember 主管指派的成员不合法。
	ErrAgentGroupInvalidMember = errors.New("invalid supervisor member target")
	// ErrAgentGroupDuplicateDelegation 主管重复指派已成功完成的相同成员任务。
	ErrAgentGroupDuplicateDelegation = errors.New("duplicate completed supervisor delegation")
	// ErrInvalidConversationProject 会话项目请求不合法。
	ErrInvalidConversationProject = apperr.New("request.invalid_conversation_project", "invalid conversation project")
	// ErrInvalidFileReference 文件引用无效。
	ErrInvalidFileReference = apperr.New("file.invalid_reference", "invalid file reference")
	// ErrInvalidFileName 文件名不合法。
	ErrInvalidFileName = apperr.New("file.invalid_name", "invalid file name")
	// ErrFileNotFound 文件不存在。
	ErrFileNotFound = apperr.New("file.not_found", "file not found")
	// ErrFileShareNotFound 文件分享不存在、已撤销、已过期或源文件不可用。
	ErrFileShareNotFound = errors.New("file share not found")
	// ErrFileInUse 文件正在被头像、知识库等资源使用。
	ErrFileInUse = apperr.NewMasked("file.in_use", "file is in use", "file in use")
	// ErrStorageQuotaExceeded 文件配额超限。
	ErrStorageQuotaExceeded = repository.ErrStorageQuotaExceeded
	// ErrFileTooLarge 文件过大。
	ErrFileTooLarge = apperr.New("file.too_large", "file too large")
	// ErrMIMEBlocked 文件类型不被允许。
	ErrMIMEBlocked = apperr.NewMasked("file.type_blocked", "file type is not allowed", "mime blocked")
	// ErrDangerousMIMEType 危险文件类型不被允许。
	ErrDangerousMIMEType = apperr.NewMasked("file.type_blocked", "file type is not allowed", "dangerous file type not allowed")
	// ErrFileProcessingNotReady 文件处理尚未就绪。
	ErrFileProcessingNotReady = apperr.NewMasked("file.not_ready", "file processing is not ready", "file processing not ready")
	// ErrFileTooLargeForFullContext 文件过大，无法全文注入。
	ErrFileTooLargeForFullContext = apperr.NewMasked("file.too_large_for_context", "file is too large for full context", "file too large for full context")
	// ErrEmbeddingUnavailable 当前未配置可用 embedding，无法处理大文档 / RAG。
	ErrEmbeddingUnavailable = apperr.NewMasked("file.embedding_unavailable", "embedding is unavailable for current file capability", "embedding unavailable")
	// ErrInvalidKnowledgeBaseReference 知识库不存在、已停用或当前用户不可见。
	ErrInvalidKnowledgeBaseReference = apperr.New(MessageErrorCodeKnowledgeBaseInvalidReference, "invalid knowledge base reference")
	// ErrKnowledgeBaseUnavailable 当前未启用可用的知识库检索能力。
	ErrKnowledgeBaseUnavailable = apperr.NewMasked(MessageErrorCodeKnowledgeBaseUnavailable, "knowledge base retrieval is unavailable", "knowledge base retrieval unavailable")
	// ErrKnowledgeBaseNotReady 所选知识库尚无可检索文件。
	ErrKnowledgeBaseNotReady = apperr.NewMasked(MessageErrorCodeKnowledgeBaseNotReady, "selected knowledge base has no ready files", "knowledge base not ready")
	// ErrTooManyMessageFiles 单条消息文件数超限。
	ErrTooManyMessageFiles = apperr.NewMasked("message.too_many_files", "too many files in one message", "too many message files")
	// ErrTooManySelectedTools 单条消息选择的 MCP 工具数超限。
	ErrTooManySelectedTools = apperr.New("message.too_many_selected_tools", "too many selected tools")
	// ErrMultipleImageAttachmentProcessors 单条消息不能同时选择多个图片附件处理器。
	ErrMultipleImageAttachmentProcessors = apperr.NewMasked("message.multiple_image_processors", "select only one image attachment processor", "multiple image attachment processors selected")
	// ErrImageAttachmentProcessingFailed 图片附件处理器调用失败。
	ErrImageAttachmentProcessingFailed = apperr.NewMasked("mcp.image_processing_failed", "image processing tool failed", "image attachment processing failed")
	// ErrMultimodalDelegationFailed 系统级多模态模型委派失败。
	ErrMultimodalDelegationFailed = errors.New("multimodal delegation failed")
	// ErrTooManySelectedSkills 单条消息选择的 Skill 数超限。
	ErrTooManySelectedSkills = apperr.New("message.too_many_selected_skills", "too many selected skills")
	// ErrPlatformApprovalNotFound 平台工具写操作批准记录不存在或不属于当前用户。
	ErrPlatformApprovalNotFound = errors.New("platform tool approval not found")
	// ErrPlatformWriteDisabled 管理员已关闭平台写工具，待批准操作不得执行。
	ErrPlatformWriteDisabled = errors.New("platform tool writes disabled")
	// ErrSkillNotFound 技能不存在或当前用户不可用。
	ErrSkillNotFound = apperr.New("skill.not_found", "skill not found")
	// ErrInvalidSkillUse 技能使用入参不合法。
	ErrInvalidSkillUse = apperr.New("request.invalid_skill_use", "invalid skill use")
	// ErrInvalidMessageBranch 消息分支参数无效。
	ErrInvalidMessageBranch = apperr.New("message.invalid_branch", "invalid message branch")
	// ErrInvalidMessageContent 消息内容不合法。
	ErrInvalidMessageContent = apperr.New("message.invalid_content", "invalid message content")
	// ErrMessageNotFound 消息不存在或无权限。
	ErrMessageNotFound = apperr.New("message.not_found", "message not found")
	// ErrContextArtifactNotFound 上下文证据不存在或无权限。
	ErrContextArtifactNotFound = apperr.New("context_artifact.not_found", "context artifact not found")
	// ErrInvalidMessageFeedback 消息反馈值不合法。
	ErrInvalidMessageFeedback = apperr.New("message.invalid_feedback", "invalid message feedback")
	// ErrMessageFeedbackTargetInvalid 反馈目标消息不合法。
	ErrMessageFeedbackTargetInvalid = apperr.NewMasked("message.feedback_target_invalid", "message feedback target invalid", "invalid message feedback target")
	// ErrMessageEditTargetInvalid 编辑目标消息不合法。
	ErrMessageEditTargetInvalid = apperr.NewMasked("message.edit_target_invalid", "message edit target invalid", "invalid message edit target")
	// ErrMessageEditStateInvalid 当前消息状态不允许编辑。
	ErrMessageEditStateInvalid = apperr.NewMasked("message.edit_state_invalid", "message edit state invalid", "invalid message edit state")
	// ErrMessageForkStateInvalid 当前消息状态不允许 fork。
	ErrMessageForkStateInvalid = apperr.New("conversation.message_fork_state_invalid", "message is still generating")
	// ErrMessageForkHistoryIncomplete 消息祖先链超过安全上限或已损坏，无法完整 fork。
	ErrMessageForkHistoryIncomplete = apperr.New("conversation.message_fork_history_incomplete", "message history is too deep or incomplete")
	ErrMessageForkTargetInvalid     = apperr.New("conversation.message_fork_target_invalid", "only assistant messages can be forked")
	ErrMessageDeleteTargetInvalid   = apperr.New("conversation.message_delete_target_invalid", "only user or assistant messages can be deleted")
	ErrMessageDeleteStateInvalid    = apperr.New("conversation.message_delete_state_invalid", "message is still generating")
	ErrMessageDeleteRootInvalid     = apperr.New("conversation.message_delete_root_invalid", "cannot delete the first message")
	ErrMessageParentDeleted         = apperr.New("conversation.message_parent_deleted", "parent message was deleted")
	// ErrModelRouteNotConfigured 模型路由未配置。
	ErrModelRouteNotConfigured = apperr.NewMasked("llm.model_route_not_configured", "model route is not configured", "model route not configured")
	// ErrModelAccessDenied 当前用户无权使用此模型。
	ErrModelAccessDenied = apperr.NewMasked("llm.model_access_denied", "you do not have access to this model", "model access denied by group policy")
	// ErrUpstreamRequestFailed 上游请求失败。
	ErrUpstreamRequestFailed = errors.New("upstream request failed")
	// ErrGeneratedMediaArtifactUnavailable 上游已完成媒体生成，但结果制品暂时无法获取或校验。
	ErrGeneratedMediaArtifactUnavailable = apperr.New(MessageErrorCodeMediaArtifactUnavailable, "generated media artifact is temporarily unavailable")
	ErrMediaArtifactPending              = errors.New("generated media artifact is pending retry")
	// ErrUpstreamEmptyResponse 上游返回空响应。
	ErrUpstreamEmptyResponse = apperr.NewMasked(MessageErrorCodeUpstreamEmptyResponse, "model returned empty response", "upstream returned empty response")
	// ErrToolRunFinalAnswerMissing 工具循环结束后上游仍未产出最终回答。
	ErrToolRunFinalAnswerMissing = apperr.New("tool_run.final_answer_missing", "tool run ended without a final answer")
	// ErrMessageGenerationCanceled 用户主动停止生成。
	ErrMessageGenerationCanceled = apperr.New("conversation_run.canceled", "message generation canceled")
	// ErrMessageGenerationInterrupted 活跃生成租约消失，当前生成流无法继续恢复。
	ErrMessageGenerationInterrupted = apperr.New("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message")
	// ErrInvalidMediaGenerationTask 媒体生成任务类型或输入不合法。
	ErrInvalidMediaGenerationTask = apperr.New("media.invalid_task", "invalid media generation task")
	// ErrInvalidReasoningEffort 思考强度档位不合法。
	ErrInvalidReasoningEffort = errors.New("invalid reasoning effort")
	// ErrMediaImagePromptRequired 图片任务提示词不能为空。
	ErrMediaImagePromptRequired = apperr.New("media.image_prompt_required", "image prompt is required")
	// ErrMediaImageGenerationRejectsInputs 图片生成任务不能携带输入图。
	ErrMediaImageGenerationRejectsInputs = apperr.New("media.image_generation_rejects_inputs", "image generation does not accept input images")
	// ErrMediaImageEditInputRequired 图片编辑任务必须携带至少一张输入图。
	ErrMediaImageEditInputRequired = apperr.New("media.image_edit_input_required", "image edit requires at least one input image")
	// ErrMediaImageEditTooManyInputs 图片编辑输入图数量超限。
	ErrMediaImageEditTooManyInputs = apperr.New("media.image_edit_too_many_inputs", "too many image edit input images")
	// ErrMediaImageEditInputInvalid 图片编辑输入图不合法。
	ErrMediaImageEditInputInvalid = apperr.New("media.image_edit_input_invalid", "image edit input image is invalid")
	// ErrMediaVideoPromptRequired 视频任务提示词不能为空。
	ErrMediaVideoPromptRequired = apperr.New("media.video_prompt_required", "video prompt is required")
	// ErrMediaVideoInputInvalid 视频生成输入不合法。
	ErrMediaVideoInputInvalid = apperr.New("media.video_input_invalid", "video generation input is invalid")
	// ErrMediaVideoTooManyInputs 视频生成输入图数量超限。
	ErrMediaVideoTooManyInputs = apperr.New("media.video_too_many_inputs", "too many video generation input images")
	// ErrMediaRouteProtocolMismatch 图片任务命中的路由协议与任务类型不匹配。
	ErrMediaRouteProtocolMismatch = apperr.New("media.route_protocol_mismatch", "media route protocol does not match task")
	// ErrDuplicateMessageGenerationRun 表示客户端重复提交同一个生成 run。
	ErrDuplicateMessageGenerationRun = apperr.NewMasked("message_generation_run.already_exists", "message generation run already exists", "duplicate message generation run")
)
