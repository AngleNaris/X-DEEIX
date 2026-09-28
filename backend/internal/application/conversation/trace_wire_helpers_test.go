package conversation

import (
	"encoding/json"
	"testing"
)

// Decode the actual serialized contract to retain absent-field assertions.
func toolTraceWireCalls(t *testing.T, payload *tracePayload) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ToolCalls []map[string]any `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	return wire.ToolCalls
}

func toolTraceWireString(t *testing.T, item map[string]any, key string) string {
	t.Helper()
	value, ok := item[key].(string)
	if !ok {
		t.Fatalf("expected string field %s, got %#v", key, item[key])
	}
	return value
}
