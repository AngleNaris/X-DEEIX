package llm

import (
	"context"
	"fmt"
	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"strings"
)

// 已支持的协议常量。每个协议固定对应一个 HTTP 端点，任务能力由模型类别和路由规则约束。
const (
	AdapterOpenAIVideo            = "openai_video_generations"
	AdapterOpenAIResponses        = "openai_responses"            // POST /v1/responses
	AdapterOpenRouterChat         = "openrouter_chat_completions" // POST /v1/chat/completions（OpenRouter）
	AdapterOpenRouterResponses    = "openrouter_responses"        // POST /v1/responses（OpenRouter Responses Beta）
	AdapterOpenAIChatCompletions  = "openai_chat_completions"     // POST /v1/chat/completions
	AdapterOpenAIImageGenerations = "openai_image_generations"    // POST /v1/images/generations
	AdapterOpenAIImageEdits       = "openai_image_edits"          // POST /v1/images/edits
	AdapterAnthropicMessages      = "anthropic_messages"          // POST /v1/messages
	AdapterGoogleGenerateContent  = "google_generate_content"     // POST /v1beta/models/{model}:generateContent
	AdapterGoogleImageGeneration  = "google_image_generation"     // POST /v1beta/models/{model}:generateContent
	AdapterGeminiInteractions     = "gemini_interactions"         // POST /v1beta/interactions
	AdapterXAIResponses           = "xai_responses"               // POST /v1/responses（OpenAI 兼容）
	AdapterXAIImage               = "xai_image"                   // POST /v1/images/generations
	AdapterImageEditsJSON         = "image_edits_json"
	AdapterXAIImageEdits          = "xai_image_edits"      // POST /v1/images/edits
	AdapterXAIVideo               = "xai_video"            // POST /v1/videos/generations + GET /v1/videos/{request_id}
	AdapterXAIVideoExtensions     = "xai_video_extensions" // POST /v1/videos/extensions + GET /v1/videos/{request_id}
)

var (
	// ErrUnsupportedAdapter 表示协议没有可用适配器实现。
	ErrUnsupportedAdapter = portllm.ErrUnsupportedAdapter
	// ErrUnsupportedStream 表示协议存在但不支持真实流式输出。
	ErrUnsupportedStream = portllm.ErrUnsupportedStream
)

type transportAdapter interface {
	Name() string
	Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error)
	GenerateStream(ctx context.Context, route RouteConfig, input GenerateInput, onEvent func(GenerateStreamEvent) error) (*GenerateOutput, error)
	ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error)
}

// NormalizeAdapter 规范化协议名；空值按历史默认使用 openai_responses，未知值保留给校验层处理。
func NormalizeAdapter(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return AdapterOpenAIResponses
	}
	return value
}

// IsKnownAdapter 返回协议是否为已知值（含未实现的）。
func IsKnownAdapter(raw string) bool { return portllm.IsImplementedAdapter(raw) }

// IsImplementedAdapter 返回协议是否已有可用的传输层实现。
func IsImplementedAdapter(raw string) bool { return portllm.IsImplementedAdapter(raw) }

// SupportsStreamingAdapter 返回协议是否有真实的上游流式传输。
func SupportsStreamingAdapter(raw string) bool { return portllm.SupportsStreamingAdapter(raw) }

// SupportsMediaInputAdapter 委托 ports/llm 的同名纯函数（数据契约唯一真相在 ports）。
func SupportsMediaInputAdapter(raw string, modality string) bool {
	return portllm.SupportsMediaInputAdapter(raw, modality)
}

// SupportsImageGenerationStream 返回图片媒体协议和模型是否支持真实上游流式。
func SupportsImageGenerationStream(protocol string, model string) bool {
	switch NormalizeAdapter(protocol) {
	case portllm.AdapterOpenRouterImages:
		return true
	case AdapterOpenAIImageGenerations:
		return openAIImageGenerationModelSupportsStream(model)
	case AdapterGoogleImageGeneration:
		return true
	case AdapterGeminiInteractions:
		return true
	case AdapterOpenAIImageEdits:
		return openAIImageEditModelSupportsStream(model)
	default:
		return false
	}
}

// IsImageGenerationAdapter 返回协议是否属于独立图片生成链路。
func IsImageGenerationAdapter(raw string) bool { return portllm.IsImageGenerationAdapter(raw) }

// IsImageEditAdapter 返回协议是否属于独立图片编辑链路。
// openai_image_generations 按任务切换到 edits 端点执行编辑请求（见 openAIImageGenerationsAdapter），
// 且路由层已允许 image_edit 能力绑定该协议，因此此处必须将其视为编辑链路协议，
// 否则编辑任务路由命中后会被 StreamMediaImage 的协议门禁误杀（media.route_protocol_mismatch）。
func IsImageEditAdapter(raw string) bool { return portllm.IsImageEditAdapter(raw) }

// IsVideoGenerationAdapter 返回协议是否属于独立视频生成链路。
func IsVideoGenerationAdapter(raw string) bool { return portllm.IsVideoGenerationAdapter(raw) }

// DefaultEndpointForAdapter 返回协议对应的固定端点标识。
func DefaultEndpointForAdapter(adapter string) string {
	return portllm.DefaultEndpointForAdapter(adapter)
}

// SupportsPreviousResponseID 返回协议是否明确支持 previous_response_id 有状态续接。
// 兼容/逆向实现即使复用 Responses 形状，也不一定支持该字段；默认保持关闭。
func SupportsPreviousResponseID(adapter string) bool {
	return NormalizeAdapter(adapter) == AdapterOpenAIResponses
}

func validateAdapter(raw string) error {
	if !IsKnownAdapter(raw) {
		return fmt.Errorf("%w: %s", ErrUnsupportedAdapter, raw)
	}
	return nil
}
