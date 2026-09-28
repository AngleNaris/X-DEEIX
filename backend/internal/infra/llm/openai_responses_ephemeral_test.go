package llm

import "testing"

func TestBuildResponsesRequestBodyEnforcesEphemeralRequest(t *testing.T) {
	for _, adapter := range []string{AdapterOpenAIResponses, AdapterOpenRouterResponses, AdapterXAIResponses} {
		t.Run(adapter, func(t *testing.T) {
			input := GenerateInput{
				Messages:            []Message{{Role: "user", Content: "hello"}},
				PromptCacheKey:      "cache_should_not_be_sent",
				PreviousResponseID:  "resp_should_not_be_sent",
				ResponsesBackground: true,
				Ephemeral:           true,
				Options: map[string]any{
					"store": true, "background": true,
					"previous_response_id":   "override",
					"prompt_cache_key":       "override",
					"prompt_cache_retention": "24h",
					"prompt_cache_options":   map[string]any{"mode": "explicit"},
				},
			}
			payload := buildResponsesRequestBody(adapter, "gpt-test", input, input.Messages, nil, nil, false, nil, true)
			if store, ok := payload["store"].(bool); !ok || store {
				t.Fatalf("store = %#v, want false", payload["store"])
			}
			for _, key := range []string{"background", "previous_response_id", "prompt_cache_key", "prompt_cache_options", "prompt_cache_retention"} {
				if _, exists := payload[key]; exists {
					t.Errorf("ephemeral request contains %s", key)
				}
			}
			if input.Options["store"] != true {
				t.Fatal("request builder mutated caller options")
			}
		})
	}
}

func TestResponsesPersistentRequestRetainsManagedState(t *testing.T) {
	input := GenerateInput{PreviousResponseID: "resp_keep", ResponsesBackground: true, PromptCacheKey: "cache_keep"}
	payload := buildResponsesRequestBody(AdapterOpenAIResponses, "gpt-test", input, nil, nil, nil, false, nil, true)
	if payload["store"] != true || payload["background"] != true || payload["previous_response_id"] != "resp_keep" || payload["prompt_cache_key"] != "cache_keep" {
		t.Fatalf("persistent request state changed: %#v", payload)
	}
}
