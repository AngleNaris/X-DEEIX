package llm

import (
	"encoding/json"
	"strings"
)

// Use the requested model, never an untrusted response model or a gateway hostname.
func isDeepSeekUsageRoute(route RouteConfig) bool {
	switch NormalizeAdapter(route.Protocol) {
	case AdapterOpenAIChatCompletions, AdapterOpenAIResponses, AdapterOpenRouterChat, AdapterOpenRouterResponses:
	default:
		return false
	}
	model := strings.ToLower(strings.TrimSpace(route.UpstreamModel))
	if slash := strings.LastIndexByte(model, '/'); slash >= 0 {
		model = model[slash+1:]
	}
	return model == "deepseek" || strings.HasPrefix(model, "deepseek-")
}

func normalizeDeepSeekUsage(usage Usage) Usage {
	var raw struct {
		Hit    *int64 `json:"prompt_cache_hit_tokens"`
		Miss   *int64 `json:"prompt_cache_miss_tokens"`
		Prompt *int64 `json:"prompt_tokens"`
		Input  *int64 `json:"input_tokens"`
	}
	if json.Unmarshal([]byte(usage.RawUsageJSON), &raw) != nil || raw.Hit == nil || *raw.Hit < 0 {
		return usage
	}
	total := raw.Prompt
	if total == nil {
		total = raw.Input
	}
	if raw.Miss != nil && *raw.Miss < 0 {
		return usage
	}
	if total != nil {
		if *total < *raw.Hit || (raw.Miss != nil && *raw.Miss != *total-*raw.Hit) {
			return usage
		}
		usage.InputTokens = *total - *raw.Hit
	} else if raw.Miss != nil {
		usage.InputTokens = *raw.Miss
	} else {
		return usage
	}
	usage.CacheReadTokens = *raw.Hit
	// Cache misses are ordinary input, not cache creation/write tokens.
	return usage
}
