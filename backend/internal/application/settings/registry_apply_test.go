package settings

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"strconv"
	"testing"
)

func TestApplyFieldRetainsValueOnInvalidInput(t *testing.T) {
	cfg := config.Config{ContextWindowFallbackTokens: 128000}
	apply := applyField(func(c *config.Config) *int { return &c.ContextWindowFallbackTokens }, func(value string, fallback int) int {
		n, err := strconv.Atoi(value)
		if err != nil {
			return fallback
		}
		return n
	})
	apply(&cfg, "invalid")
	if cfg.ContextWindowFallbackTokens != 128000 {
		t.Fatal("invalid input changed field")
	}
	apply(&cfg, "64000")
	if cfg.ContextWindowFallbackTokens != 64000 {
		t.Fatal("valid input not applied")
	}
}

func TestRegistryTextParsers(t *testing.T) {
	if rawText("  prompt\n", "old") != "  prompt\n" {
		t.Fatal("raw text changed")
	}
	if trimmedText("  model\n", "old") != "model" {
		t.Fatal("text not trimmed")
	}
}
