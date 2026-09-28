package settings

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"strings"
)

// applyField retains the current field value when a parser rejects input.
func applyField[T any](field func(*config.Config) *T, parse func(string, T) T) func(*config.Config, string) {
	return func(cfg *config.Config, value string) {
		target := field(cfg)
		*target = parse(value, *target)
	}
}

func rawText(value string, _ string) string     { return value }
func trimmedText(value string, _ string) string { return strings.TrimSpace(value) }
