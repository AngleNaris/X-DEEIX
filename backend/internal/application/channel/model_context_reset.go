package channel

import (
	"encoding/json"
	"strings"
)

// clearAutomaticContextWindow preserves explicit administrator overrides.
func clearAutomaticContextWindow(raw string) (string, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil || payload == nil {
		return raw, false
	}
	mode, ok := payload["_deeixContextWindowMode"].(string)
	if !ok || !strings.EqualFold(strings.TrimSpace(mode), "auto") {
		return raw, false
	}
	for _, key := range []string{"_deeixContextWindowMode", "contextWindow", "context_window", "contextWindowTokens", "context_window_tokens"} {
		delete(payload, key)
	}
	if len(payload) == 0 {
		return "", true
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return raw, false
	}
	return string(normalized), true
}
