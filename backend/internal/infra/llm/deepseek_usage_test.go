package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeepSeekUsageOnlyNormalizesRequestedDeepSeekModels(t *testing.T) {
	const raw = `{"prompt_tokens":100,"completion_tokens":5,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"model\":\"deepseek-flash\",\"choices\":[],\"usage\":%s}\n\ndata: [DONE]\n\n", raw)
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"model":"deepseek-flash","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":%s}`, raw)
		}
	}))
	t.Cleanup(server.Close)
	client := newTestClient()
	t.Cleanup(client.CloseIdleConnections)
	for _, model := range []string{"deepseek-flash", "deepseek/deepseek-r1", "gpt-5", "claude-sonnet", "qwen3", "gemini-3"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", model, stream), func(t *testing.T) {
				route := RouteConfig{Protocol: AdapterOpenAIChatCompletions, BaseURL: server.URL, UpstreamModel: model}
				wantInput, wantCache := int64(100), int64(0)
				if model == "deepseek-flash" || model == "deepseek/deepseek-r1" {
					wantInput, wantCache = 20, 80
				}
				check := func(u Usage) {
					t.Helper()
					if u.InputTokens != wantInput || u.CacheReadTokens != wantCache || u.CacheWriteTokens != 0 {
						t.Fatalf("usage = %+v", u)
					}
				}
				input := GenerateInput{Messages: []Message{{Role: "user", Content: "hello"}}}
				if stream {
					events := 0
					output, err := client.GenerateStream(context.Background(), route, input, func(event GenerateStreamEvent) error {
						if event.Usage.RawUsageJSON != "" {
							check(event.Usage)
							events++
						}
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					check(output.Usage)
					if events != 1 {
						t.Fatalf("usage events = %d", events)
					}
				} else {
					output, err := client.Generate(context.Background(), route, input)
					if err != nil {
						t.Fatal(err)
					}
					check(output.Usage)
				}
			})
		}
	}
}

func TestDeepSeekUsageBoundaryValues(t *testing.T) {
	for _, tt := range []struct {
		raw          string
		input, cache int64
	}{
		{`{"prompt_tokens":100,"prompt_cache_hit_tokens":100,"prompt_cache_miss_tokens":0}`, 0, 100},
		{`{"prompt_tokens":100,"prompt_cache_hit_tokens":0,"prompt_cache_miss_tokens":100}`, 100, 0},
		{`{"prompt_tokens":100,"prompt_cache_hit_tokens":101}`, 100, 0},
		{`{"prompt_tokens":100,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":99}`, 100, 0},
		{`{"prompt_tokens":100}`, 100, 0},
	} {
		u := normalizeDeepSeekUsage(Usage{InputTokens: 100, RawUsageJSON: tt.raw})
		if u.InputTokens != tt.input || u.CacheReadTokens != tt.cache {
			t.Fatalf("%s: %+v", tt.raw, u)
		}
		if again := normalizeDeepSeekUsage(u); again != u {
			t.Fatal("not idempotent")
		}
	}
	if isDeepSeekUsageRoute(RouteConfig{Protocol: AdapterAnthropicMessages, UpstreamModel: "deepseek-flash"}) {
		t.Fatal("unexpected DeepSeek fields on Anthropic protocol")
	}
}
