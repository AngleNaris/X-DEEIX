package conversation

import (
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

type messageUsageAccumulator struct {
	observedUsage                   llm.Usage
	inputObserved                   bool
	estimatedUnobservedInputTokens  int64
	currentCallEstimatedInputTokens int64

	estimatedUnobservedOutputTokens    int64
	estimatedUnobservedReasoningTokens int64
	// completedObservedUsage 是当前调用开始时的观测快照，与 observedUsage 的差值即当前调用已上报的部分。
	completedObservedUsage   llm.Usage
	currentCallVisibleText   strings.Builder
	currentCallReasoningText strings.Builder
}

func (a *messageUsageAccumulator) beginCall(estimatedInputTokens int64) {
	a.currentCallEstimatedInputTokens = max(estimatedInputTokens, 0)
	a.completedObservedUsage = a.observedUsage
	a.currentCallVisibleText.Reset()
	a.currentCallReasoningText.Reset()
}

// recordCallVisibleText 记录当前调用已产出的可见文本，供上游未上报输出用量时预估。
func (a *messageUsageAccumulator) recordCallVisibleText(text string) {
	a.currentCallVisibleText.WriteString(text)
}

// recordCallReasoningText 记录当前调用已产出的思考文本，供上游未上报输出用量时预估。
func (a *messageUsageAccumulator) recordCallReasoningText(text string) {
	a.currentCallReasoningText.WriteString(text)
}

// finishCall 结束当前调用：上报了对应侧用量则丢弃本次预估，否则把预估计入未观测部分。
func (a *messageUsageAccumulator) finishCall(observedInput bool, outputObserved ...bool) {
	observedOutput := a.observedUsage.OutputTokens > a.completedObservedUsage.OutputTokens || a.observedUsage.ReasoningTokens > a.completedObservedUsage.ReasoningTokens
	if len(outputObserved) > 0 {
		observedOutput = outputObserved[0]
	}
	if observedInput {
		a.markInputObserved()
	} else {
		a.estimatedUnobservedInputTokens += a.currentCallEstimatedInputTokens
		a.currentCallEstimatedInputTokens = 0
	}
	if !observedOutput {
		outputTokens, reasoningTokens := estimateOutputUsage(0, 0, a.currentCallVisibleText.String(), a.currentCallReasoningText.String())
		a.estimatedUnobservedOutputTokens += outputTokens
		a.estimatedUnobservedReasoningTokens += reasoningTokens
	}
	a.currentCallVisibleText.Reset()
	a.currentCallReasoningText.Reset()
}

func (a *messageUsageAccumulator) addObservedUsage(delta llm.Usage) llm.Usage {
	if delta == (llm.Usage{}) {
		return a.observedUsage
	}
	a.observedUsage = addLLMUsage(a.observedUsage, delta)
	if delta.HasObservedInput() {
		a.markInputObserved()
	}
	return a.observedUsage
}

func (a *messageUsageAccumulator) setObservedUsage(usage llm.Usage) {
	a.observedUsage = usage
	if usage.HasObservedInput() {
		a.markInputObserved()
	}
}

func (a *messageUsageAccumulator) markInputObserved() {
	a.inputObserved = true
	a.currentCallEstimatedInputTokens = 0
}

func (a *messageUsageAccumulator) usage() llm.Usage {
	return a.observedUsage
}

func (a *messageUsageAccumulator) interruptedInputTokens() int64 {
	return a.observedUsage.InputTokens + a.estimatedUnobservedInputTokens + a.currentCallEstimatedInputTokens
}

// interruptedOutputTokens 返回中断时计费的输出与思考 token。已完成调用采用观测值加未上报调用的
// 预估；被中断的当前调用取其已上报部分与尾段文本预估的较大者，避免上游按块上报时重复计费。
func (a *messageUsageAccumulator) interruptedOutputTokens() (int64, int64) {
	currentOutputTokens, currentReasoningTokens := estimateOutputUsage(
		max(a.observedUsage.OutputTokens-a.completedObservedUsage.OutputTokens, 0),
		max(a.observedUsage.ReasoningTokens-a.completedObservedUsage.ReasoningTokens, 0),
		a.currentCallVisibleText.String(),
		a.currentCallReasoningText.String(),
	)
	return a.completedObservedUsage.OutputTokens + a.estimatedUnobservedOutputTokens + currentOutputTokens,
		a.completedObservedUsage.ReasoningTokens + a.estimatedUnobservedReasoningTokens + currentReasoningTokens
}

// estimateOutputUsage 用已产出文本补齐一次调用的输出与思考用量：上游拆分上报了思考 token 时两侧
// 分别取较大者；只上报了合并输出时把思考文本并入输出预估，避免重复计费；完全未上报时按文本预估。
func estimateOutputUsage(observedOutputTokens int64, observedReasoningTokens int64, visibleText string, reasoningText string) (int64, int64) {
	estimatedOutputTokens := estimateTokens(visibleText)
	estimatedReasoningTokens := estimateTokens(reasoningText)
	switch {
	case observedReasoningTokens > 0:
		return resolveObservedOrHigherEstimatedTokens(observedOutputTokens, estimatedOutputTokens),
			resolveObservedOrHigherEstimatedTokens(observedReasoningTokens, estimatedReasoningTokens)
	case observedOutputTokens > 0:
		return resolveObservedOrHigherEstimatedTokens(observedOutputTokens, estimatedOutputTokens+estimatedReasoningTokens), 0
	default:
		return estimatedOutputTokens, estimatedReasoningTokens
	}
}

// effectiveInputTokens 返回本条消息最终计费的非缓存输入。只要有调用上报过输入侧用量，
// 非缓存输入为 0（提示词全部命中缓存）也如实采用；仅在完全没有观测值时才回退到规划预估。
func (a *messageUsageAccumulator) effectiveInputTokens(promptFallback int64) int64 {
	inputTokens := a.observedUsage.InputTokens + a.estimatedUnobservedInputTokens
	if a.inputObserved || inputTokens > 0 {
		return inputTokens
	}
	return max(promptFallback, 0)
}

// effectiveOutputTokens 返回本条消息最终计费的输出与思考 token：观测值加上未上报用量调用的文本预估。
func (a *messageUsageAccumulator) effectiveOutputTokens() (int64, int64) {
	return a.observedUsage.OutputTokens + a.estimatedUnobservedOutputTokens,
		a.observedUsage.ReasoningTokens + a.estimatedUnobservedReasoningTokens
}

// billedUsage 返回本条消息到目前为止按计费口径汇总的用量：输入与输出侧取观测值加未上报调用的预估，
// 缓存读写只有观测值。工具循环再次调用上游前据此校验预算，与最终账单口径一致。
func (a *messageUsageAccumulator) billedUsage() llm.Usage {
	usage := a.observedUsage
	usage.InputTokens = a.interruptedInputTokens()
	usage.OutputTokens, usage.ReasoningTokens = a.effectiveOutputTokens()
	return usage
}

func hasObservedInputUsage(usage llm.Usage) bool {
	return usage.InputTokens > 0 || usage.CacheReadTokens > 0 || usage.CacheWriteTokens > 0
}

func resolveObservedOrEstimatedOutputTokens(observedTokens int64, assistantText string) int64 {
	return resolveObservedOrEstimatedTokens(observedTokens, estimateTokens(assistantText))
}

func resolveObservedOrEstimatedTokens(observedTokens int64, estimatedTokens int64) int64 {
	if observedTokens > 0 {
		return observedTokens
	}
	if estimatedTokens > 0 {
		return estimatedTokens
	}
	return 0
}

func resolveObservedOrHigherEstimatedOutputTokens(observedTokens int64, assistantText string) int64 {
	return resolveObservedOrHigherEstimatedTokens(observedTokens, estimateTokens(assistantText))
}

func resolveObservedOrHigherEstimatedTokens(observedTokens int64, estimatedTokens int64) int64 {
	if estimatedTokens > observedTokens {
		return estimatedTokens
	}
	if observedTokens > 0 {
		return observedTokens
	}
	return 0
}

// estimateBillableInputTokens includes retained upstream context for Responses continuations.
func estimateBillableInputTokens(input llm.GenerateInput, fullMessages []llm.Message) int64 {
	if strings.TrimSpace(input.PreviousResponseID) == "" {
		return estimateGenerateInputTokens(input)
	}
	return estimateToolFollowUpInputTokens(input, fullMessages)
}

func maxPromptTokenEstimate(values ...int64) int64 {
	var result int64
	for _, value := range values {
		if value > result {
			result = value
		}
	}
	return result
}

func estimateGenerateInputTokens(input llm.GenerateInput) int64 {
	tokens := estimatePromptTokens(input.Messages)
	if instructions := strings.TrimSpace(input.Instructions); instructions != "" {
		tokens += estimateTokens(instructions) + 4
	}
	if !input.DisableTools {
		tokens += estimateToolDefinitionTokens(input.Tools)
	}
	return tokens
}

func estimateToolDefinitionTokens(tools []llm.ToolDefinition) int64 {
	if len(tools) == 0 {
		return 0
	}
	var tokens int64 = 2
	for _, tool := range tools {
		tokens += estimateTokens(tool.Name)
		tokens += estimateTokens(tool.Description)
		tokens += estimateTokens(string(tool.InputSchema))
		tokens += 12
	}
	return tokens
}

// toolFollowUpContextBudget keeps legacy callers compatible while accepting the runtime fallback.
func toolFollowUpContextBudget(modelName, capabilitiesJSON string, fallbackContextWindow ...int) int {
	fallback := 0
	if len(fallbackContextWindow) > 0 {
		fallback = fallbackContextWindow[0]
	}
	return domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback(modelName, capabilitiesJSON, fallback)
}

// resolveToolResultTokenBudget 计算当前用户轮次的全部工具结果可使用的模型输入预算。
// 新批次先使用该上限，回灌前再对同轮全部结果统一分配，不额外透支有效上下文。
func resolveToolResultTokenBudget(
	generateInput llm.GenerateInput,
	messages []llm.Message,
	pendingAssistant llm.Message,
	modelName string,
	capabilitiesJSON string,
	fallbackContextWindow ...int,
) int64 {
	budgetMessages := toolResultPayloadPlaceholders(prioritizeCurrentToolMessages(messages))
	placeholderResults := make([]llm.ToolResult, 0, len(pendingAssistant.ToolCalls))
	for _, call := range pendingAssistant.ToolCalls {
		placeholderResults = append(placeholderResults, llm.ToolResult{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			OutputJSON: "{}",
		})
	}
	budgetMessages = append(
		budgetMessages,
		pendingAssistant,
		llm.Message{Role: "tool", ToolResults: placeholderResults},
	)
	available := int64(toolFollowUpContextBudget(modelName, capabilitiesJSON, fallbackContextWindow...)) -
		estimateToolFollowUpInputTokens(generateInput, budgetMessages)
	if available < 0 {
		return 0
	}
	return available
}

// rebalanceToolFollowUpResults 在完整工具回灌请求超预算时，统一压缩当前轮的全部工具结果。
func rebalanceToolFollowUpResults(
	generateInput llm.GenerateInput,
	messages []llm.Message,
	modelName string,
	capabilitiesJSON string,
	fallbackContextWindow ...int,
) ([]llm.Message, bool) {
	effectiveBudget := int64(toolFollowUpContextBudget(modelName, capabilitiesJSON, fallbackContextWindow...))
	if estimateToolFollowUpInputTokens(generateInput, messages) <= effectiveBudget {
		return messages, false
	}

	_, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex < 0 {
		return messages, false
	}
	fixedMessages := toolResultPayloadPlaceholders(messages)
	resultBudget := effectiveBudget - estimateToolFollowUpInputTokens(generateInput, fixedMessages)
	if resultBudget < 0 {
		resultBudget = 0
	}

	type resultRef struct {
		messageIndex int
		resultIndex  int
	}
	result := append([]llm.Message(nil), messages...)
	refs := make([]resultRef, 0)
	slots := make([]toolExecutionSlot, 0)
	for messageIndex := currentUserIndex + 1; messageIndex < len(result); messageIndex++ {
		if len(result[messageIndex].ToolResults) == 0 {
			continue
		}
		result[messageIndex].ToolResults = append([]llm.ToolResult(nil), result[messageIndex].ToolResults...)
		for resultIndex, toolResult := range result[messageIndex].ToolResults {
			refs = append(refs, resultRef{messageIndex: messageIndex, resultIndex: resultIndex})
			slots = append(slots, toolExecutionSlot{result: toolResult})
		}
	}
	if len(slots) == 0 {
		return messages, false
	}

	enforceToolResultAggregateBudget(slots, resultBudget)
	changed := false
	for index, ref := range refs {
		if result[ref.messageIndex].ToolResults[ref.resultIndex] != slots[index].result {
			changed = true
			result[ref.messageIndex].ToolResults[ref.resultIndex] = slots[index].result
		}
	}
	if !changed {
		return messages, false
	}
	return result, true
}

// toolResultPayloadPlaceholders 保留工具结果的协议结构，但移除可变正文以计算固定上下文开销。
func toolResultPayloadPlaceholders(messages []llm.Message) []llm.Message {
	result := append([]llm.Message(nil), messages...)
	for messageIndex := range result {
		if len(result[messageIndex].ToolResults) == 0 {
			continue
		}
		placeholders := make([]llm.ToolResult, 0, len(result[messageIndex].ToolResults))
		for _, toolResult := range result[messageIndex].ToolResults {
			placeholders = append(placeholders, llm.ToolResult{
				ToolCallID: toolResult.ToolCallID,
				ToolName:   toolResult.ToolName,
				OutputJSON: "{}",
				Status:     toolResult.Status,
			})
		}
		result[messageIndex].ToolResults = placeholders
	}
	return result
}

// trimToolFollowUpHistory 仅在工具回灌请求超预算时删除最老的完整历史轮次。
func trimToolFollowUpHistory(
	generateInput llm.GenerateInput,
	messages []llm.Message,
	modelName string,
	capabilitiesJSON string,
	fallbackContextWindow ...int,
) ([]llm.Message, bool) {
	effectiveBudget := int64(toolFollowUpContextBudget(modelName, capabilitiesJSON, fallbackContextWindow...))
	estimatedTokens := estimateToolFollowUpInputTokens(generateInput, messages)
	if estimatedTokens <= effectiveBudget {
		return messages, false
	}

	systemEnd, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex <= systemEnd {
		return messages, false
	}
	for cutFrom := systemEnd; cutFrom < currentUserIndex; cutFrom++ {
		estimatedTokens -= estimateMessageTokens(messages[cutFrom])
		nextIndex := cutFrom + 1
		if nextIndex < currentUserIndex && messages[nextIndex].Role != "user" {
			continue
		}
		if estimatedTokens <= effectiveBudget || nextIndex == currentUserIndex {
			trimmed := make([]llm.Message, 0, systemEnd+len(messages)-nextIndex)
			trimmed = append(trimmed, messages[:systemEnd]...)
			trimmed = append(trimmed, messages[nextIndex:]...)
			return trimmed, true
		}
	}
	return messages, false
}

// prioritizeCurrentToolMessages 返回系统指令和当前用户轮次，供工具结果计算最大可用预算。
func prioritizeCurrentToolMessages(messages []llm.Message) []llm.Message {
	systemEnd, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex <= systemEnd {
		return append([]llm.Message(nil), messages...)
	}
	result := make([]llm.Message, 0, systemEnd+len(messages)-currentUserIndex)
	result = append(result, messages[:systemEnd]...)
	result = append(result, messages[currentUserIndex:]...)
	return result
}

// toolHistoryBounds 定位系统前缀结束位置和当前轮用户消息。
func toolHistoryBounds(messages []llm.Message) (int, int) {
	systemEnd := 0
	for systemEnd < len(messages) && messages[systemEnd].Role == "system" {
		systemEnd++
	}
	currentUserIndex := -1
	for index := len(messages) - 1; index >= systemEnd; index-- {
		if messages[index].Role == "user" {
			currentUserIndex = index
			break
		}
	}
	return systemEnd, currentUserIndex
}

// estimateToolFollowUpInputTokens 按全量请求形状估算工具回灌输入。
func estimateToolFollowUpInputTokens(generateInput llm.GenerateInput, messages []llm.Message) int64 {
	budgetMessages := messages
	if strings.TrimSpace(generateInput.Instructions) != "" {
		_, budgetMessages = extractOpenAIResponsesInstructions(messages)
	}
	budgetInput := generateInput
	budgetInput.Messages = budgetMessages
	budgetInput.PreviousResponseID = ""
	return estimateGenerateInputTokens(budgetInput)
}

