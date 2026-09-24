package settings

import "testing"

// TestValidateModelOptionPathsAcceptsImageEditsJSON 锁定 F1 修复：
// image_edits_json 必须与 openai_image_edits 同为合法策略键，前后端运行时一致。
func TestValidateModelOptionPathsAcceptsImageEditsJSON(t *testing.T) {
	payload := `{"default":["temperature"],"image_edits_json":["quality","size"],"openai_video_generations":["duration"]}`
	if err := validateModelOptionPathsJSON(payload, "chat.model_option_allowed_paths"); err != nil {
		t.Fatalf("image_edits_json should be accepted, got: %v", err)
	}
}

// TestValidateModelOptionPathsRejectsUnknownProtocol 保证校验仍拒绝拼写错误的协议键。
func TestValidateModelOptionPathsRejectsUnknownProtocol(t *testing.T) {
	payload := `{"default":["temperature"],"openai_imag_edits":["quality"]}`
	if err := validateModelOptionPathsJSON(payload, "chat.model_option_allowed_paths"); err == nil {
		t.Fatal("misspelled protocol key should be rejected")
	}
}
