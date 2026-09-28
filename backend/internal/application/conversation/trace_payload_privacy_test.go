package conversation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTracePayloadRetainsQueryLengthWithoutQueryText(t *testing.T) {
	payload := &tracePayload{QueryChars: len([]rune("私密查询"))}
	cloned := payload.clone()
	raw, err := json.Marshal(cloned)
	if err != nil {
		t.Fatal(err)
	}
	if cloned.QueryChars != 4 || strings.Contains(string(raw), `"query":`) || strings.Contains(string(raw), "私密") {
		t.Fatalf("query privacy or clone contract violated: %s", raw)
	}
	if !strings.Contains(string(raw), `"query_chars":4`) {
		t.Fatalf("query length lost: %s", raw)
	}
	var legacy tracePayload
	if err := json.Unmarshal([]byte(`{"query":"legacy","query_chars":6}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Query != "legacy" || legacy.QueryChars != 6 {
		t.Fatal("legacy trace decoding regressed")
	}
}
