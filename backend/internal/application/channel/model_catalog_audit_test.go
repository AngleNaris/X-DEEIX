package channel

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

// TestKnownProtocolMatchesLLMAdapter 锁定 F5 修复：
// channel 侧协议清单必须与 llm.IsKnownAdapter 保持一致（委托实现防漂移）。
func TestKnownProtocolMatchesLLMAdapter(t *testing.T) {
	protocols := []string{
		"openai_responses", "openrouter_chat_completions", "openrouter_responses",
		"openai_chat_completions", "anthropic_messages", "google_generate_content",
		"xai_responses", "openai_image_generations", "openai_image_edits",
		"openai_video_generations", "image_edits_json", "google_image_generation",
		"gemini_interactions", "xai_image", "xai_image_edits",
		"xai_video", "xai_video_extensions",
	}
	for _, protocol := range protocols {
		if !isKnownProtocol(protocol) {
			t.Errorf("isKnownProtocol(%q) = false, want true", protocol)
		}
		if !llm.IsKnownAdapter(protocol) {
			t.Errorf("llm.IsKnownAdapter(%q) = false, want true", protocol)
		}
	}
	if isKnownProtocol("not_a_protocol") {
		t.Error("isKnownProtocol(not_a_protocol) = true, want false")
	}
}

// TestImageEditAllowedMatchesAdapterGate 锁定生图修复同族约束：
// 矩阵允许 image_edit 的协议必须全部通过 llm.IsImageEditAdapter。
func TestImageEditAllowedMatchesAdapterGate(t *testing.T) {
	protocols := []string{
		"openai_image_edits", "openai_image_generations", "google_image_generation",
		"gemini_interactions", "xai_image_edits", "image_edits_json",
	}
	for _, protocol := range protocols {
		if !isProtocolAllowedForKind(modelKindImageEdit, protocol) {
			t.Errorf("isProtocolAllowedForKind(image_edit, %q) = false, want true", protocol)
		}
		if !llm.IsImageEditAdapter(protocol) {
			t.Errorf("llm.IsImageEditAdapter(%q) = false, want true", protocol)
		}
	}
	if isProtocolAllowedForKind(modelKindImageEdit, "xai_image") {
		t.Error("xai_image must not be allowed for image_edit (generation-only)")
	}
}

// TestMediaOnlyProtocolFlagsChatRoutedMedia 锁定 F6 观测：纯媒体协议必须被标记，聊天协议不标记。
func TestMediaOnlyProtocolFlagsChatRoutedMedia(t *testing.T) {
	for _, protocol := range []string{"openai_image_generations", "xai_image", "xai_video_extensions", "openai_video_generations"} {
		if !isMediaOnlyRouteProtocol(protocol) {
			t.Errorf("isMediaOnlyRouteProtocol(%q) = false, want true", protocol)
		}
	}
	for _, protocol := range []string{"openai_responses", "openai_chat_completions", "xai_responses", "gemini_interactions"} {
		if isMediaOnlyRouteProtocol(protocol) {
			t.Errorf("isMediaOnlyRouteProtocol(%q) = true, want false", protocol)
		}
	}
}
