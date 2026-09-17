package conversation

import (
	"context"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type messageRoutePromptInput struct {
	UserContent string
	// UserID 用于解析系统提示词模板变量（{{language}}/{{username}}）。
	UserID                   uint
	ConversationID           uint
	AppendUserContent        bool
	ProjectSystemPrompt      string
	RoleSystemPrompt         string
	HTMLVisualPromptEnabled  bool
	ReasoningContentPassback bool
	DomainMessages           []model.Message
	StableAttachments        []AttachmentInput
	AttachmentImports        []attachmentImportPath
	DynamicContext           userContextInput
	PreferencePrompt         string
	SkillPrompts             *skillPrompts
	ToolRuntime              selectedToolRuntime
	SkipImageAttachments     bool
	Config                   config.Config
}

func withMessageRouteReasoningPassbackOptions(
	options map[string]interface{},
	inputOptions map[string]interface{},
	route *channel.ResolvedRoute,
	reasoningContentPassback bool,
	messages []llm.Message,
) map[string]interface{} {
	if route == nil || !shouldApplyReasoningPassbackRequestOptions(
		reasoningContentPassback,
		route.ReasoningPassbackRequestOptions,
		messages,
	) {
		return options
	}
	return withReasoningPassbackRequestOptions(
		options,
		route.ReasoningPassbackRequestOptions,
		inputOptions,
		route.ModelCapabilitiesJSON,
	)
}

func (s *Service) buildMessageRoutePrompt(ctx context.Context, route *channel.ResolvedRoute, input messageRoutePromptInput) (PromptPlan, error) {
	routeMessages := s.applyContextTokenBudget(
		input.DomainMessages,
		route.UpstreamModel,
		route.ModelCapabilitiesJSON,
		input.ReasoningContentPassback,
	)
	historyMessages := historyMessagesFromDomain(routeMessages, historyMessageOptions{
		ReasoningContentPassback: input.ReasoningContentPassback,
	})
	if !input.SkipImageAttachments {
		var err error
		supportsVision := modelSupportsVision(route.PlatformModelName, route.ModelCapabilitiesJSON)
		historyMessages, err = s.injectConversationImageContext(
			ctx,
			historyMessages,
			routeMessages,
			input.StableAttachments,
			input.Config,
			supportsVision,
			input.DynamicContext.HistoricalArtifacts,
		)
		if err != nil {
			return PromptPlan{}, err
		}
	}
	var replayErr error
	historyMessages, replayErr = s.replayMessageToolHistory(ctx, input.UserID, input.ConversationID, routeMessages, historyMessages, input.ReasoningContentPassback)
	if replayErr != nil {
		return PromptPlan{}, replayErr
	}
	if len(historyMessages) == 0 || input.AppendUserContent {
		historyMessages = append(historyMessages, llm.Message{Role: "user", Content: input.UserContent})
	}

	assembler := NewContextAssembler(int64(input.Config.ContextMaxInputTokens))
	systemPrompt := resolveMessageSystemPromptInjection(
		input.Config,
		route,
		input.ProjectSystemPrompt,
		input.RoleSystemPrompt,
		input.HTMLVisualPromptEnabled,
		s.resolveSystemPromptVars(ctx, input.UserID),
	)
	if systemPrompt.Content != "" {
		if systemPrompt.InlineToUser {
			historyMessages = inlineSystemPromptIntoLatestUserMessage(historyMessages, systemPrompt.Content)
		} else {
			assembler.Add(ContextSlot{Kind: SlotSystemPrompt, Content: systemPrompt.Content, Required: true})
		}
	}
	if input.PreferencePrompt != "" {
		assembler.Add(ContextSlot{Kind: SlotPreference, Content: input.PreferencePrompt, Required: true})
	}
	baseMessages, _ := assembler.Assemble(historyMessages)
	plan := buildPromptPlan(ctx, promptPlanInput{
		BaseMessages:      baseMessages,
		StableAttachments: input.StableAttachments,
		AttachmentImports: input.AttachmentImports,
		DynamicContext:    input.DynamicContext,
		SkillPrompts:      input.SkillPrompts,
		ToolRuntime:       input.ToolRuntime,
		Config:            input.Config,
		StoreProvider:     s.storeProvider,
	})
	// Include replayed tool payloads in the input budget; trim whole user turns, never half a tool pair.
	plan.Messages, _ = trimToolFollowUpHistory(llm.GenerateInput{Tools: input.ToolRuntime.definitions}, plan.Messages, route.UpstreamModel, route.ModelCapabilitiesJSON)
	plan.Trace.TotalTokenEstimate = estimatePromptTokens(plan.Messages)
	return plan, nil
}
