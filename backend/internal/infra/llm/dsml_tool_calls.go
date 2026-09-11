package llm

import (
	"encoding/json"
	"errors"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var errDeepSeekDSMLToolCallsIncomplete = errors.New("deepseek dsml tool calls ended before a complete tool call envelope")

var (
	dsmlTokenRE           = `(?:｜DSML｜|｜｜DSML｜｜|\|\|DSML\|\||\|DSML\|)`
	dsmlToolCallsBlockRE  = regexp.MustCompile(`(?is)<\s*` + dsmlTokenRE + `\s*tool_calls\s*>(.*?)</\s*` + dsmlTokenRE + `\s*tool_calls\s*>`)
	dsmlInvokeRE          = regexp.MustCompile(`(?is)<\s*` + dsmlTokenRE + `\s*invoke\b([^>]*)>(.*?)</\s*` + dsmlTokenRE + `\s*invoke\s*>`)
	dsmlParameterRE       = regexp.MustCompile(`(?is)<\s*` + dsmlTokenRE + `\s*parameter\b([^>]*)>(.*?)</\s*` + dsmlTokenRE + `\s*parameter\s*>`)
	dsmlAttributeRE       = regexp.MustCompile(`(?is)([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*("([^"]*)"|'([^']*)')`)
	plainToolCallsBlockRE = regexp.MustCompile(`(?is)\\?\s*<\s*tool_calls\s*>(.*?)\\?\s*</\s*tool_calls\s*>`)
)

// textEncodedToolCallMode 控制 DeepSeek DSML 文本工具调用的解析行为。
type textEncodedToolCallMode int

const (
	// textEncodedToolCallsInactive 路由未启用 DSML 文本解析。
	textEncodedToolCallsInactive textEncodedToolCallMode = iota
	// textEncodedToolCallsStripOnly 工具已禁用（强制收尾/阶段合并轮）：只剥离文本中的
	// DSML 标记，不生成 ToolCall，避免残留调用把本应收尾的轮次误判为工具调用。
	textEncodedToolCallsStripOnly
	// textEncodedToolCallsActive 完整解析：剥离标记并生成结构化 ToolCall 供循环执行。
	textEncodedToolCallsActive
	// textEncodedToolCallsGenericStripOnly 使用普通 <tool_calls> JSON 协议，
	// 仅剥离文本中的调用，不生成 ToolCall。
	textEncodedToolCallsGenericStripOnly
	// textEncodedToolCallsGenericActive 使用普通 <tool_calls> JSON 协议并执行调用。
	textEncodedToolCallsGenericActive
)

// resolveTextEncodedToolCallMode 决定给定路由与调用形态下的 DSML 解析模式。
// DeepSeek V4 即使收到"禁用工具"指令仍可能在文本里输出 DSML 标记，
// 因此工具禁用轮次必须剥离标记但不生成 ToolCall。
func resolveTextEncodedToolCallMode(route RouteConfig, disableTools bool) textEncodedToolCallMode {
	if deepSeekTextEncodedToolCallsEnabled(route) {
		if disableTools {
			return textEncodedToolCallsStripOnly
		}
		return textEncodedToolCallsActive
	}
	if !genericTextEncodedToolCallsEnabled(route) {
		return textEncodedToolCallsInactive
	}
	if disableTools {
		return textEncodedToolCallsGenericStripOnly
	}
	return textEncodedToolCallsGenericActive
}

// applyTextEncodedToolCalls 将 DeepSeek V4 文本编码的工具调用转换为内部结构化 ToolCall。
// 这段兼容只在路由层显式判定为 DeepSeek Chat Completions 时调用，避免影响其他 OpenAI-compatible 模型。
// 工具禁用轮（strip-only）只剥离标记；若剥离后模型仍试图调用工具，置 TextToolCallsStripped
// 供上层判断（例如触发阶段合并续轮），而不是静默丢失。
func applyTextEncodedToolCalls(output *GenerateOutput, mode textEncodedToolCallMode) {
	if output == nil || mode == textEncodedToolCallsInactive {
		return
	}
	cleanText, toolCalls, ok := parseTextEncodedToolCalls(output.Text, mode)
	if ok {
		output.Text = cleanText
	}
	if textEncodedToolCallsAreActive(mode) {
		if ok {
			output.ToolCalls = append(output.ToolCalls, toolCalls...)
		}
		return
	}
	// strip-only：残留的未闭合 DSML 标记属于工具禁用轮的模型噪音，直接移除。
	if stripped := stripRemainingTextToolCallMarkers(&output.Text); stripped {
		output.TextToolCallsStripped = true
	}
	if ok {
		output.TextToolCallsStripped = true
	}
}

// stripRemainingTextToolCallMarkers 移除文本中第一个文本工具调用起始标记之后的内容。
// 完整块已被解析剥离，此处剩余的标记必然未闭合或畸形，属于模型噪音。
func stripRemainingTextToolCallMarkers(text *string) bool {
	if text == nil {
		return false
	}
	lower := strings.ToLower(*text)
	markers := []string{
		"<tool_calls",
		"\\<tool_calls",
		"<｜dsml｜tool_calls",
		"<｜｜dsml｜｜tool_calls",
		"<||dsml||tool_calls",
		"<|dsml|tool_calls",
	}
	index := -1
	for _, marker := range markers {
		if found := strings.Index(lower, marker); found >= 0 && (index < 0 || found < index) {
			index = found
		}
	}
	if index < 0 {
		return false
	}
	*text = strings.TrimSpace((*text)[:index])
	return true
}

// deepSeekTextEncodedToolCallsEnabled 判断当前路由是否需要启用 DeepSeek DSML 文本工具调用解析。
func deepSeekTextEncodedToolCallsEnabled(route RouteConfig) bool {
	if NormalizeAdapter(route.Protocol) != AdapterOpenAIChatCompletions {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(route.UpstreamModel))
	baseURL := strings.ToLower(strings.TrimSpace(route.BaseURL))
	return strings.Contains(model, "deepseek") || strings.Contains(baseURL, "deepseek")
}

func stripRemainingDSMLToolCallMarkers(text *string) bool {
	return stripRemainingTextToolCallMarkers(text)
}

func genericTextEncodedToolCallsEnabled(route RouteConfig) bool {
	switch NormalizeAdapter(route.Protocol) {
	case AdapterOpenAIChatCompletions, AdapterOpenRouterChat:
		return true
	default:
		return normalizeEndpoint(route.Endpoint) == EndpointChatCompletions
	}
}

func textEncodedToolCallsAreActive(mode textEncodedToolCallMode) bool {
	return mode == textEncodedToolCallsActive || mode == textEncodedToolCallsGenericActive
}

func parseTextEncodedToolCalls(text string, mode textEncodedToolCallMode) (string, []ToolCall, bool) {
	cleanText := text
	toolCalls := make([]ToolCall, 0)
	parsed := false
	if mode == textEncodedToolCallsActive || mode == textEncodedToolCallsGenericActive || mode == textEncodedToolCallsGenericStripOnly {
		if clean, calls, ok := parsePlainToolCalls(cleanText, len(toolCalls)); ok {
			cleanText = clean
			toolCalls = append(toolCalls, calls...)
			parsed = true
		}
	}
	if mode == textEncodedToolCallsActive || mode == textEncodedToolCallsStripOnly {
		if clean, calls, ok := parseDSMLToolCalls(cleanText); ok {
			cleanText = clean
			for index := range calls {
				calls[index].ToolCallID = "dsml_call_" + strconv.Itoa(len(toolCalls)+index+1)
			}
			toolCalls = append(toolCalls, calls...)
			parsed = true
		}
	}
	if !parsed {
		return text, nil, false
	}
	return strings.TrimSpace(cleanText), toolCalls, true
}

func parsePlainToolCalls(text string, offset int) (string, []ToolCall, bool) {
	matches := plainToolCallsBlockRE.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil, false
	}
	var clean strings.Builder
	toolCalls := make([]ToolCall, 0)
	last := 0
	for _, match := range matches {
		blockStart, blockEnd := match[0], match[1]
		contentStart, contentEnd := match[2], match[3]
		var items []map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(text[contentStart:contentEnd])), &items); err != nil {
			continue
		}
		blockCalls := make([]ToolCall, 0, len(items))
		for _, item := range items {
			name := strings.TrimSpace(getString(item["name"]))
			if function := asMap(item["function"]); name == "" {
				name = strings.TrimSpace(getString(function["name"]))
			}
			if name == "" {
				continue
			}
			arguments := normalizeJSONString(item["arguments"])
			if arguments == "" {
				arguments = normalizeJSONString(item["input"])
			}
			if arguments == "" {
				arguments = "{}"
			}
			toolType := strings.TrimSpace(getString(item["type"]))
			if toolType == "" {
				toolType = "function"
			}
			callID := strings.TrimSpace(getString(item["id"]))
			if callID == "" {
				callID = "text_call_" + strconv.Itoa(offset+len(toolCalls)+len(blockCalls)+1)
			}
			blockCalls = append(blockCalls, ToolCall{
				ToolCallID:    callID,
				ToolType:      toolType,
				ToolName:      name,
				ArgumentsJSON: arguments,
				Status:        "requested",
			})
		}
		if len(blockCalls) == 0 {
			continue
		}
		clean.WriteString(text[last:blockStart])
		last = blockEnd
		toolCalls = append(toolCalls, blockCalls...)
	}
	if len(toolCalls) == 0 {
		return text, nil, false
	}
	clean.WriteString(text[last:])
	return strings.TrimSpace(clean.String()), toolCalls, true
}

// parseDSMLToolCalls 解析 DeepSeek V4 以 DSML 文本片段返回的工具调用。
// 解析成功时只移除完整 tool_calls 块；无法形成合法工具调用时保留原文，交由调用方按普通文本或格式错误处理。
func parseDSMLToolCalls(text string) (string, []ToolCall, bool) {
	matches := dsmlToolCallsBlockRE.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil, false
	}

	var clean strings.Builder
	toolCalls := make([]ToolCall, 0)
	last := 0
	for _, match := range matches {
		blockStart, blockEnd := match[0], match[1]
		contentStart, contentEnd := match[2], match[3]
		blockToolCalls := parseDSMLInvokeToolCalls(text[contentStart:contentEnd], len(toolCalls))
		if len(blockToolCalls) == 0 {
			continue
		}
		clean.WriteString(text[last:blockStart])
		last = blockEnd
		toolCalls = append(toolCalls, blockToolCalls...)
	}
	if len(toolCalls) == 0 {
		return text, nil, false
	}
	clean.WriteString(text[last:])
	return strings.TrimSpace(clean.String()), toolCalls, true
}

// parseDSMLInvokeToolCalls 将 DSML invoke 节点映射为本地函数工具调用。
func parseDSMLInvokeToolCalls(content string, offset int) []ToolCall {
	matches := dsmlInvokeRE.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make([]ToolCall, 0, len(matches))
	for _, match := range matches {
		attrs := parseDSMLAttributes(match[1])
		toolName := strings.TrimSpace(attrs["name"])
		if toolName == "" {
			continue
		}
		args := parseDSMLParameters(match[2])
		if args == nil {
			continue
		}
		argsJSON, err := json.Marshal(args)
		if err != nil {
			argsJSON = []byte("{}")
		}
		result = append(result, ToolCall{
			ToolCallID:    "dsml_call_" + strconv.Itoa(offset+len(result)+1),
			ToolType:      "function",
			ToolName:      toolName,
			ArgumentsJSON: string(argsJSON),
			Status:        "requested",
		})
	}
	return result
}

// parseDSMLParameters 按 DeepSeek DSML 的 string 标记还原参数类型。
// 参数名重复时放弃本次 invoke，避免同名覆盖造成工具入参歧义。
func parseDSMLParameters(content string) map[string]interface{} {
	params := map[string]interface{}{}
	for _, match := range dsmlParameterRE.FindAllStringSubmatch(content, -1) {
		attrs := parseDSMLAttributes(match[1])
		name := strings.TrimSpace(attrs["name"])
		if name == "" {
			continue
		}
		if _, exists := params[name]; exists {
			return nil
		}
		value := strings.TrimSpace(html.UnescapeString(match[2]))
		if strings.EqualFold(strings.TrimSpace(attrs["string"]), "true") {
			params[name] = value
			continue
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			params[name] = decoded
			continue
		}
		params[name] = value
	}
	return params
}

// parseDSMLAttributes 解析 DSML 标签属性，属性名统一小写以匹配上游格式波动。
func parseDSMLAttributes(raw string) map[string]string {
	attrs := map[string]string{}
	for _, match := range dsmlAttributeRE.FindAllStringSubmatch(raw, -1) {
		value := match[3]
		if value == "" {
			value = match[4]
		}
		attrs[strings.ToLower(strings.TrimSpace(match[1]))] = html.UnescapeString(value)
	}
	return attrs
}
