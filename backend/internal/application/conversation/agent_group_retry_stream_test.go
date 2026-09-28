package conversation

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestAgentGroupRetryStreamIDIsolation(t *testing.T) {
	first := agentGroupRetryStreamID("run", "retry-1")
	if first != agentGroupRetryStreamID("run", "retry-1") {
		t.Fatal("retry replay lost stable lease identity")
	}
	for _, other := range []string{
		agentGroupRetryStreamID("run", "retry-2"),
		agentGroupRetryStreamID("other-run", "retry-1"),
		"run",
	} {
		if first == other {
			t.Fatal("distinct executions share lease identity")
		}
	}
	if agentGroupRetryStreamID("ab", "c") == agentGroupRetryStreamID("a", "bc") {
		t.Fatal("ambiguous key framing")
	}
	id := agentGroupRetryStreamID(strings.Repeat("r", 64), strings.Repeat("q", 128))
	if len(id) != 64 {
		t.Fatalf("stream ID exceeds contract: %d", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatal(err)
	}
}
