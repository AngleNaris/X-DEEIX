package settings

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"testing"
)

func TestNormalizeContextPolicyBounds(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		window, percent, wantWindow, wantPercent int
	}{
		{"invalid-low", -1, -1, config.DefaultContextWindowFallbackTokens, config.DefaultContextCompactTriggerPercent},
		{"invalid-high", config.MaxContextWindowFallbackTokens + 1, 96, config.DefaultContextWindowFallbackTokens, config.DefaultContextCompactTriggerPercent},
		{"percent-below-min", 128000, 9, 128000, config.DefaultContextCompactTriggerPercent},
		{"disabled", 128000, 0, 128000, 0},
		{"minimum", config.MinContextWindowFallbackTokens, 10, config.MinContextWindowFallbackTokens, 10},
		{"maximum", config.MaxContextWindowFallbackTokens, 95, config.MaxContextWindowFallbackTokens, 95},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{ContextWindowFallbackTokens: tc.window, ContextCompactTriggerPercent: tc.percent}
			(&RuntimeSettings{}).normalizeConfig(&cfg)
			if cfg.ContextWindowFallbackTokens != tc.wantWindow || cfg.ContextCompactTriggerPercent != tc.wantPercent {
				t.Fatalf("got window=%d percent=%d; want %d %d", cfg.ContextWindowFallbackTokens, cfg.ContextCompactTriggerPercent, tc.wantWindow, tc.wantPercent)
			}
		})
	}
}
