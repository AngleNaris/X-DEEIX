package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

type mcpToolCallBinding struct {
	Config       mcp.CallConfig
	ServerID     uint
	ServerName   string
	ToolName     string
	PriceNanousd int64
}

type selectedToolRuntime struct {
	mcpBindings          map[string]mcpToolCallBinding
	definitions          []llm.ToolDefinition
	nameMap              map[string]string
	mcpConfigs           map[string]mcp.CallConfig
	schemas              map[string]json.RawMessage
	attachmentProcessor  *selectedAttachmentProcessor
	multimodalAnalyzer   *selectedMultimodalAnalyzer
	credentialSecrets    *credentialSecretRefStore
	platformEntries      map[string]platformToolEntry // 平台内置工具（本地执行，无 MCP 配置）
	platformDefinitions  []llm.ToolDefinition
	platformNameMap      map[string]string
	authorizedMCPTools   map[string]authorizedMCPTool
	authorizedMCPOrder   []string
	authorizedMCPServers map[uint]authorizedMCPServer
	mcpActivation        *mcpActivationState
	onMCPActivation      func(context.Context, []uint) error
	// supportsVision 标记当前模型是否原生支持图片输入。文本模型（无 vision）拿不到
	// 视觉工具返回的图片像素，激活披露与工具描述需明确警告，避免无效工具循环。
	supportsVision bool
}

const mcpActivateServerToolName = "mcp_activate_server"

var mcpActivateServerInputSchema = json.RawMessage(`{
	"type":"object",
	"properties":{"server_id":{"type":"integer","minimum":1,"description":"Authorized MCP server ID to activate"}},
	"required":["server_id"]
}`)

type authorizedMCPTool struct {
	serverName   string
	priceNanousd int64
	serverID     uint
	definition   llm.ToolDefinition
	toolName     string
	config       mcp.CallConfig
	schema       json.RawMessage
}

type authorizedMCPServer struct {
	id          uint
	name        string
	description string
}

type mcpActivationState struct {
	mu        sync.Mutex
	serverIDs map[uint]struct{}
}

func newMCPActivationState(serverIDs []uint) *mcpActivationState {
	state := &mcpActivationState{serverIDs: make(map[uint]struct{}, len(serverIDs))}
	for _, serverID := range serverIDs {
		if serverID != 0 {
			state.serverIDs[serverID] = struct{}{}
		}
	}
	return state
}

func (s *mcpActivationState) activeServerIDs() []uint {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedMCPServerIDs(s.serverIDs)
}

func (s *mcpActivationState) retainAuthorizedServers(servers map[uint]authorizedMCPServer) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for serverID := range s.serverIDs {
		if _, ok := servers[serverID]; !ok {
			delete(s.serverIDs, serverID)
		}
	}
}

func sortedMCPServerIDs(items map[uint]struct{}) []uint {
	result := make([]uint, 0, len(items))
	for serverID := range items {
		result = append(result, serverID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

type selectedAttachmentProcessor struct {
	serverName     string
	priceNanousd   int64
	toolID         uint
	serverID       uint
	modelName      string
	toolName       string
	displayName    string
	mode           string // image / audio / file（决定注入哪些附件）
	argument       string
	encoding       string
	promptArgument string
	config         mcp.CallConfig
	schema         json.RawMessage
}

func addProjectToolDefinitions(runtime *selectedToolRuntime) {
	if runtime == nil || runtime.nameMap == nil || runtime.schemas == nil {
		return
	}
	definitions := []llm.ToolDefinition{
		{Name: "project_list_files", Description: "列出当前项目文件。需要了解项目文件树、确认文件是否存在或获取文件标识时必须调用，不得凭上下文猜测。", InputSchema: json.RawMessage(`{"type":"object","properties":{"prefix":{"type":"string","description":"可选的项目内相对路径前缀"}}}`)},
		{Name: "project_read_file", Description: "读取当前项目文件内容。回答、分析或修改现有项目文件前必须调用；聊天上下文、记忆或代码块不能替代实际读取。", InputSchema: json.RawMessage(`{"type":"object","properties":{"file":{"type":"string","description":"文件 public_id 或项目内相对路径"}},"required":["file"]}`)},
		{Name: "project_search_files", Description: "搜索当前项目文件的实际内容。需要定位实现、符号或文本时必须调用，不得用记忆或推测替代搜索。", InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"要搜索的非空文本"}},"required":["query"]}`)},
		{Name: "project_write_file", Description: "创建或完整覆盖当前项目文件。用户要求创建、生成、保存或覆盖项目文件时必须调用；仅在回复中给出代码块、补丁或声称已完成不等于文件写入。路径已存在时会覆盖并递增版本。", InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"项目内相对路径，不得为绝对路径或包含上级目录穿越"},"content":{"type":"string","description":"要写入的完整文件内容"}},"required":["path","content"]}`)},
		{Name: "project_patch_file", Description: "像 IDE 一样对已有项目文件做片段编辑：提供文件中唯一且连续的原始片段 old 与替换文本 new，或多处修改时用 patches 数组一次提交多个片段（全部成功才写入，任一 old 未命中则整体失败并提示片段序号）。用户要求编辑、修复或更新已有文件时优先使用本工具，不要用 project_write_file 重写整个文件。调用前必须实际读取目标文件，回复中的 diff 不能替代修改。", InputSchema: json.RawMessage(`{"type":"object","properties":{"file":{"type":"string","description":"文件 public_id 或项目内相对路径"},"old":{"type":"string","description":"单片段模式：文件中唯一且连续的原始文本"},"new":{"type":"string","description":"单片段模式：替换后的文本"},"patches":{"type":"array","description":"多片段模式：一次提交多处修改，全部命中才写入","items":{"type":"object","properties":{"old":{"type":"string","description":"文件中唯一且连续的原始文本"},"new":{"type":"string","description":"替换后的文本"}},"required":["old","new"]}}},"required":["file"]}`)},
		{Name: "project_delete_file", Description: "删除当前项目文件。用户要求删除项目文件时必须调用；口头确认、空内容覆盖或建议用户自行删除均不能替代此工具。", InputSchema: json.RawMessage(`{"type":"object","properties":{"file":{"type":"string","description":"文件 public_id 或项目内相对路径"}},"required":["file"]}`)},
		{Name: "project_create_archive", Description: "创建当前项目 ZIP 归档。归档通过界面的下载按钮交付给用户，调用成功后只需说明归档已生成；严禁输出任何下载 URL、链接或路径，直接链接因需要鉴权而无法访问。", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	}
	for _, definition := range definitions {
		runtime.definitions = append(runtime.definitions, definition)
		runtime.nameMap[definition.Name] = definition.Name
		runtime.schemas[definition.Name] = definition.InputSchema
	}
}

func (r selectedToolRuntime) withoutProjectTools() selectedToolRuntime {
	projectNames := map[string]struct{}{
		"project_list_files": {}, "project_read_file": {}, "project_search_files": {},
		"project_write_file": {}, "project_patch_file": {}, "project_delete_file": {}, "project_create_archive": {},
	}
	definitions := make([]llm.ToolDefinition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		if _, ok := projectNames[definition.Name]; !ok {
			definitions = append(definitions, definition)
		}
	}
	r.definitions = definitions
	for name := range projectNames {
		delete(r.nameMap, name)
		delete(r.mcpConfigs, name)
		delete(r.mcpBindings, name)
		delete(r.schemas, name)
	}
	return r
}
func injectMCPToolGuidance(messages []llm.Message, runtime selectedToolRuntime, customPrompt string) []llm.Message {
	if len(runtime.definitions) == 0 {
		return messages
	}

	content := defaultMCPToolGuidancePrompt(runtime)
	if custom := strings.TrimSpace(customPrompt); custom != "" {
		content += "\n\n# custom_tool_use\n" + custom
	}
	if len(runtime.platformEntries) > 0 {
		content = content + "\n\n" + platformToolGuidancePrompt()
	}
	if guidance := runtime.multimodalAnalyzerGuidance(); guidance != "" {
		content = content + "\n\n" + guidance
	}

	insertAt := 0
	for insertAt < len(messages) && messages[insertAt].Role == "system" {
		insertAt++
	}
	next := make([]llm.Message, 0, len(messages)+1)
	next = append(next, messages[:insertAt]...)
	next = append(next, llm.Message{Role: "system", Content: content})
	next = append(next, messages[insertAt:]...)
	return next
}

func defaultMCPToolGuidancePrompt(runtime selectedToolRuntime) string {
	var builder strings.Builder
	builder.WriteString("# tool_use\n")
	builder.WriteString("- Tools are declared separately via the API schema; follow that schema exactly.\n")
	builder.WriteString("- Use tools only for external, realtime, private, or explicitly requested data.\n")
	builder.WriteString("- Use the fewest useful calls; each call must add new information.\n")
	builder.WriteString("- Do not repeat an identical failed call. Adjust arguments, use another tool, or answer from available evidence.\n")
	builder.WriteString("- MCP server activation is an intermediate step. Continue the original task in the automatic same-run follow-up; never ask the user to send another message just to use activated tools.\n")
	builder.WriteString("- If tools fail or lack enough data, state the gap in the final answer.\n")
	builder.WriteString("- Do not expose raw tool JSON, internal fields, or tool logs unless the user asks.\n")
	if hasProjectToolDefinitions(runtime) {
		builder.WriteString("- Project file operations are real side effects: listing, reading, searching, creating, overwriting, patching, or deleting project files MUST use the corresponding project_* tool when available.\n")
		builder.WriteString("- Never substitute a code block, diff, explanation, memory, or claim of completion for a required project file tool call. Read an existing file before modifying it, and report success only after the tool succeeds.\n")
		builder.WriteString("- Prefer project_patch_file with focused old/new snippets (or the patches array for multiple edits) when modifying existing files; reserve project_write_file for creating files or explicit full rewrites.\n")
	}
	return strings.TrimSpace(builder.String())
}

func hasProjectToolDefinitions(runtime selectedToolRuntime) bool {
	for _, definition := range runtime.definitions {
		switch definition.Name {
		case "project_list_files", "project_read_file", "project_search_files", "project_write_file", "project_patch_file", "project_delete_file", "project_create_archive":
			return true
		}
	}
	return false
}

func summarizeToolInputSchema(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return ""
	}
	properties, _ := schema["properties"].(map[string]interface{})
	if len(properties) == 0 {
		return "无需参数"
	}
	required := map[string]struct{}{}
	if items, ok := schema["required"].([]interface{}); ok {
		for _, item := range items {
			if name, ok := item.(string); ok && strings.TrimSpace(name) != "" {
				required[strings.TrimSpace(name)] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		prop, _ := properties[name].(map[string]interface{})
		fieldType := schemaFieldType(prop)
		label := name
		if fieldType != "" {
			label = fmt.Sprintf("%s:%s", name, fieldType)
		}
		if _, ok := required[name]; ok {
			label += " 必填"
		}
		parts = append(parts, label)
	}
	if len(parts) > 6 {
		parts = append(parts[:6], fmt.Sprintf("等 %d 个字段", len(parts)))
	}
	return "参数 " + strings.Join(parts, "，")
}

func schemaFieldType(prop map[string]interface{}) string {
	if len(prop) == 0 {
		return ""
	}
	if value, ok := prop["type"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if items, ok := prop["type"].([]interface{}); ok && len(items) > 0 {
		types := make([]string, 0, len(items))
		for _, item := range items {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				types = append(types, strings.TrimSpace(value))
			}
		}
		if len(types) > 0 {
			return strings.Join(types, "|")
		}
	}
	if _, ok := prop["enum"].([]interface{}); ok {
		return "enum"
	}
	return ""
}

func (s *Service) resolveSelectedToolRuntime(ctx context.Context, toolIDs []uint) (selectedToolRuntime, error) {
	return s.resolveSelectedToolRuntimeForModel(ctx, toolIDs, true)
}

// resolveSelectedToolRuntimeForModel 按模型 vision 能力解析工具运行时：文本模型（无 vision）
// 的激活披露与工具描述会附带"无法查看图片"警告。
func (s *Service) resolveSelectedToolRuntimeForModel(ctx context.Context, toolIDs []uint, supportsVision bool) (selectedToolRuntime, error) {
	return s.resolveSelectedToolRuntimeWithActivation(ctx, toolIDs, nil, nil, supportsVision)
}

func (s *Service) resolveSelectedToolRuntimeWithActivation(
	ctx context.Context,
	toolIDs []uint,
	activation *mcpActivationState,
	onActivation func(context.Context, []uint) error,
	supportsVision bool,
) (selectedToolRuntime, error) {
	if activation == nil {
		activation = newMCPActivationState(nil)
	}
	result := selectedToolRuntime{
		definitions:          make([]llm.ToolDefinition, 0, len(toolIDs)+8),
		nameMap:              map[string]string{},
		mcpConfigs:           map[string]mcp.CallConfig{},
		schemas:              map[string]json.RawMessage{},
		authorizedMCPTools:   map[string]authorizedMCPTool{},
		authorizedMCPServers: map[uint]authorizedMCPServer{},
		mcpActivation:        activation,
		onMCPActivation:      onActivation,
		supportsVision:       supportsVision,
	}
	addProjectToolDefinitions(&result)
	if len(toolIDs) > 0 && s.cfg.Snapshot().MCPEnable {
		if s.mcpRepo == nil {
			return selectedToolRuntime{}, fmt.Errorf("resolve selected MCP tools: repository unavailable")
		}
		if err := s.resolveMCPToolRuntime(ctx, toolIDs, &result); err != nil {
			return selectedToolRuntime{}, err
		}
	}
	// 平台内置工具（本地执行）：读工具受 platform_tools.enabled 控制，写工具另需 write_enabled。
	if err := s.appendPlatformToolRuntime(ctx, &result); err != nil {
		return selectedToolRuntime{}, err
	}
	activation.retainAuthorizedServers(result.authorizedMCPServers)
	return result.visibleRuntime(), nil
}

// resolveMCPToolRuntime 解析用户勾选的 MCP 工具（原 resolveSelectedToolRuntime 主体）。
func (s *Service) resolveMCPToolRuntime(ctx context.Context, toolIDs []uint, result *selectedToolRuntime) error {
	tools, err := s.mcpRepo.ListToolsByIDs(ctx, uniqueToolIDs(toolIDs))
	if err != nil {
		return fmt.Errorf("resolve selected MCP tools: %w", err)
	}
	if len(tools) == 0 {
		return nil
	}

	cfg := s.cfg.Snapshot()
	outboundPolicy := cfg.TrustedOutboundPolicy()
	usedNames := map[string]int{
		mcpActivateServerToolName:       1,
		systemMultimodalAnalyzeToolName: 1,
		"project_list_files":            1, "project_read_file": 1, "project_search_files": 1,
		"project_write_file": 1, "project_patch_file": 1, "project_delete_file": 1, "project_create_archive": 1,
	}
	serverCache := map[uint]*domainmcp.Server{}
	for _, tool := range tools {
		if tool.Status != "active" {
			continue
		}
		isAttachmentProcessor := domainmcp.IsValidAttachmentMode(strings.TrimSpace(tool.AttachmentInputMode))
		server, ok := serverCache[tool.ServerID]
		if !ok {
			server, err = s.mcpRepo.GetServer(ctx, tool.ServerID)
			if err != nil {
				return fmt.Errorf("resolve MCP server %d: %w", tool.ServerID, err)
			}
			if server == nil || server.Status != "active" {
				if isAttachmentProcessor {
					return fmt.Errorf("%w: processor server is unavailable", ErrImageAttachmentProcessingFailed)
				}
				continue
			}
			if validateErr := security.ValidateOutboundHTTPURL(server.BaseURL, outboundPolicy); validateErr != nil {
				if isAttachmentProcessor {
					return fmt.Errorf("%w: processor server URL is not allowed", ErrImageAttachmentProcessingFailed)
				}
				continue
			}
			serverCache[tool.ServerID] = server
		}
		baseModelName := llm.NormalizeToolName(tool.Name)
		if strings.TrimSpace(baseModelName) == "" {
			continue
		}
		schema := json.RawMessage(strings.TrimSpace(tool.InputSchemaJSON))
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		token, err := secretbox.DecryptString(cfg.DataEncryptionKey, server.AuthTokenEnc)
		if err != nil {
			if isAttachmentProcessor {
				return fmt.Errorf("%w: processor credentials are unavailable", ErrImageAttachmentProcessingFailed)
			}
			continue
		}
		headers := parseMCPHeaders(server.HeadersJSON)
		callConfig := mcp.CallConfig{
			BaseURL:   server.BaseURL,
			AuthToken: token,
			TimeoutMS: cfg.MCPToolTimeoutSeconds * 1000,
			Headers:   headers,
		}
		if isAttachmentProcessor {
			if bindErr := result.bindAttachmentProcessor(selectedAttachmentProcessor{
				serverName:     strings.TrimSpace(server.Name),
				priceNanousd:   tool.PriceNanousd,
				toolID:         tool.ID,
				serverID:       tool.ServerID,
				modelName:      baseModelName,
				toolName:       tool.Name,
				displayName:    firstNonEmptyString(tool.DisplayName, tool.Name),
				mode:           strings.ToLower(strings.TrimSpace(tool.AttachmentInputMode)),
				argument:       strings.TrimSpace(tool.AttachmentArgument),
				encoding:       strings.TrimSpace(tool.AttachmentEncoding),
				promptArgument: strings.TrimSpace(tool.AttachmentPromptArgument),
				config:         callConfig,
				schema:         schema,
			}); bindErr != nil {
				return bindErr
			}
			result.authorizedMCPServers[tool.ServerID] = authorizedMCPServer{
				id:          tool.ServerID,
				name:        strings.TrimSpace(server.Name),
				description: strings.TrimSpace(server.Description),
			}
			continue

		}
		modelName := uniqueModelToolName(baseModelName, usedNames)
		if modelName == "" {
			continue
		}
		definition := llm.ToolDefinition{
			Name:        modelName,
			Description: strings.TrimSpace(tool.Description),
			InputSchema: schema,
		}
		result.authorizedMCPTools[modelName] = authorizedMCPTool{
			serverName:   strings.TrimSpace(server.Name),
			priceNanousd: tool.PriceNanousd,
			serverID:     tool.ServerID,
			definition:   definition,
			toolName:     tool.Name,
			config:       callConfig,
			schema:       schema,
		}
		result.authorizedMCPOrder = append(result.authorizedMCPOrder, modelName)
		result.authorizedMCPServers[tool.ServerID] = authorizedMCPServer{
			id:          tool.ServerID,
			name:        strings.TrimSpace(server.Name),
			description: strings.TrimSpace(server.Description),
		}
	}
	return nil
}

func (r selectedToolRuntime) visibleRuntime() selectedToolRuntime {
	r.definitions = append([]llm.ToolDefinition(nil), r.platformDefinitions...)
	r.nameMap = make(map[string]string, len(r.platformEntries)+len(r.authorizedMCPTools)+2)
	r.schemas = make(map[string]json.RawMessage, len(r.platformEntries)+len(r.authorizedMCPTools)+2)
	r.mcpConfigs = make(map[string]mcp.CallConfig, len(r.authorizedMCPTools))
	r.mcpBindings = make(map[string]mcpToolCallBinding, len(r.authorizedMCPTools)+1)
	if p := r.attachmentProcessor; p != nil {
		r.mcpBindings[p.modelName] = mcpToolCallBinding{Config: p.config, ServerID: p.serverID, ServerName: p.serverName, ToolName: p.toolName, PriceNanousd: p.priceNanousd}
	}
	for modelName, entry := range r.platformEntries {
		executionName := strings.TrimSpace(r.platformNameMap[modelName])
		if executionName == "" {
			executionName = entry.definition.Name
		}
		r.nameMap[modelName] = executionName
		r.schemas[modelName] = entry.definition.InputSchema
	}
	if r.multimodalAnalyzer != nil {
		definition := r.multimodalAnalyzer.toolDefinition()
		r.definitions = append(r.definitions, definition)
		r.nameMap[definition.Name] = systemMultimodalAnalyzeToolName
		r.schemas[definition.Name] = definition.InputSchema
	}
	if len(r.authorizedMCPServers) == 0 {
		return r
	}
	r.definitions = append(r.definitions, llm.ToolDefinition{
		Name:        mcpActivateServerToolName,
		Description: r.mcpActivationDescription(),
		InputSchema: mcpActivateServerInputSchema,
	})
	r.nameMap[mcpActivateServerToolName] = mcpActivateServerToolName
	r.schemas[mcpActivateServerToolName] = mcpActivateServerInputSchema
	active := make(map[uint]struct{})
	if r.mcpActivation != nil {
		for _, serverID := range r.mcpActivation.activeServerIDs() {
			active[serverID] = struct{}{}
		}
	}
	for _, modelName := range r.authorizedMCPOrder {
		tool, ok := r.authorizedMCPTools[modelName]
		if !ok {
			continue
		}
		if _, ok := active[tool.serverID]; !ok {
			continue
		}
		definition := tool.definition
		if !r.supportsVision {
			definition.Description = appendTextOnlyModelToolWarning(definition.Description)
		}
		r.definitions = append(r.definitions, definition)
		r.nameMap[modelName] = tool.toolName
		r.schemas[modelName] = tool.schema
		r.mcpConfigs[modelName] = tool.config
		r.mcpBindings[modelName] = mcpToolCallBinding{Config: tool.config, ServerID: tool.serverID, ServerName: tool.serverName, ToolName: tool.toolName, PriceNanousd: tool.priceNanousd}
	}
	return r
}

// appendTextOnlyModelToolWarning 为文本模型（无 vision）的工具描述附加警告：
// 视觉类工具返回的图片像素对它不可见，应优先使用返回文字的工具或请求用户提供文字。
func appendTextOnlyModelToolWarning(description string) string {
	const warning = "\n\nIMPORTANT: You are a text-only model and cannot view image content returned by tools. " +
		"Tools that return images (for example image viewing or visualization tools) will not help you " +
		"because the pixels are invisible to you. If you need text inside an image, use a tool that " +
		"returns text (such as OCR), or ask the user to provide the text directly."
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return strings.TrimSpace(warning)
	}
	return trimmed + warning
}

func (r selectedToolRuntime) mcpActivationDescription() string {
	serverIDs := make([]uint, 0, len(r.authorizedMCPServers))
	for serverID := range r.authorizedMCPServers {
		serverIDs = append(serverIDs, serverID)
	}
	sort.Slice(serverIDs, func(i, j int) bool { return serverIDs[i] < serverIDs[j] })
	var builder strings.Builder
	builder.WriteString("Activate one MCP server authorized for this run. The backend immediately starts another model step in this same run with its selected tool schemas. Authorized servers:\n")
	for _, serverID := range serverIDs {
		server := r.authorizedMCPServers[serverID]
		description := server.description
		if description == "" {
			description = "No administrator description provided."
		}
		fmt.Fprintf(&builder, "- server_id=%d; name=%s; description=%s\n", server.id, server.name, description)
	}
	builder.WriteString("Only activate a server when its described capability is needed. After activation, continue the user's original task in the automatic same-run follow-up and call the newly visible tools when needed. Never ask the user to send another message or merely report that activation succeeded. Do not guess or invoke undisclosed MCP tool names.")
	if !r.supportsVision {
		builder.WriteString(appendTextOnlyModelToolWarning(""))
	}
	return strings.TrimSpace(builder.String())
}

func (r *selectedToolRuntime) activateMCPServer(ctx context.Context, serverID uint) (bool, error) {
	if r == nil || serverID == 0 {
		return false, fmt.Errorf("invalid MCP server id")
	}
	if _, ok := r.authorizedMCPServers[serverID]; !ok {
		return false, fmt.Errorf("MCP server %d is not authorized for this run", serverID)
	}
	if r.mcpActivation == nil {
		return false, fmt.Errorf("MCP activation state is unavailable")
	}
	r.mcpActivation.mu.Lock()
	defer r.mcpActivation.mu.Unlock()
	if _, ok := r.mcpActivation.serverIDs[serverID]; ok {
		return false, nil
	}
	next := make(map[uint]struct{}, len(r.mcpActivation.serverIDs)+1)
	for activeServerID := range r.mcpActivation.serverIDs {
		next[activeServerID] = struct{}{}
	}
	next[serverID] = struct{}{}
	serverIDs := sortedMCPServerIDs(next)
	if r.onMCPActivation != nil {
		if err := r.onMCPActivation(ctx, serverIDs); err != nil {
			return false, err
		}
	}
	r.mcpActivation.serverIDs[serverID] = struct{}{}
	return true, nil
}

func (r *selectedToolRuntime) bindAttachmentProcessor(processor selectedAttachmentProcessor) error {
	if r.attachmentProcessor != nil {
		return ErrMultipleImageAttachmentProcessors
	}
	r.attachmentProcessor = &processor
	return nil
}

func (r selectedToolRuntime) attachmentProcessorActive() bool {
	if r.attachmentProcessor == nil || r.attachmentProcessor.serverID == 0 || r.mcpActivation == nil {
		return false
	}
	for _, serverID := range r.mcpActivation.activeServerIDs() {
		if serverID == r.attachmentProcessor.serverID {
			return true
		}
	}
	return false
}

func (r selectedToolRuntime) withoutAttachmentProcessor() selectedToolRuntime {
	r.attachmentProcessor = nil
	return r.visibleRuntime()
}

func (r selectedToolRuntime) withoutDefinitions() selectedToolRuntime {
	r.definitions = nil
	r.nameMap = nil
	r.mcpConfigs = nil
	r.mcpBindings = nil
	r.schemas = nil
	r.attachmentProcessor = nil
	r.multimodalAnalyzer = nil
	r.credentialSecrets = nil
	r.platformEntries = nil
	r.platformDefinitions = nil
	r.platformNameMap = nil
	r.authorizedMCPTools = nil
	r.authorizedMCPOrder = nil
	r.authorizedMCPServers = nil
	return r
}

func uniqueToolIDs(items []uint) []uint {
	seen := make(map[uint]struct{}, len(items))
	result := make([]uint, 0, len(items))
	for _, item := range items {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func uniqueModelToolName(base string, used map[string]int) string {
	value := strings.TrimSpace(base)
	if value == "" {
		return ""
	}
	count := used[value]
	used[value] = count + 1
	if count == 0 {
		return value
	}
	suffix := "_" + strconv.Itoa(count+1)
	if len(value)+len(suffix) > 64 {
		value = value[:64-len(suffix)]
	}
	return value + suffix
}

func parseMCPHeaders(raw string) map[string]string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]string{}
	}
	payload := map[string]string{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(payload))
	for key, item := range payload {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		result[headerKey] = strings.TrimSpace(item)
	}
	return result
}

