package conversation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCredentialScrubPreservesTypedTracePayload(t *testing.T) {
	secret := "test-only-private-credential-123456"
	writes := []credentialWrite{{Name: "api", Value: secret, Ref: "credential://api"}}
	payload := &tracePayload{Reason: secret, QueryChars: 4, Reasoning: &traceReasoning{Signature: secret}}
	scrubbed := scrubCredentialTracePayload(payload, writes, writes)
	raw, err := json.Marshal(scrubbed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatal("typed trace retains credential")
	}
	if scrubbed.QueryChars != 4 || scrubbed.Reasoning == nil {
		t.Fatal("scrubbing lost typed fields")
	}
	if payload.Reason != secret {
		t.Fatal("scrubbing mutated original snapshot")
	}
	reasoning := scrubCredentialTracePayload(&traceReasoning{Signature: secret}, writes, writes)
	if strings.Contains(reasoning.Signature, secret) {
		t.Fatal("pending reasoning retains credential")
	}
	var absent *tracePayload
	if scrubCredentialTracePayload(absent, writes, writes) != nil {
		t.Fatal("nil payload must stay nil")
	}
	legacy := scrubCredentialTracePayload(map[string]interface{}{"secret": secret}, writes, writes)
	if strings.Contains(legacy["secret"].(string), secret) {
		t.Fatal("legacy payload retains credential")
	}
}
