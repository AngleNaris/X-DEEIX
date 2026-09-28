package channel

import "strings"

// truncateMessage limits diagnostic previews without splitting UTF-8 characters.
func truncateMessage(message string, limit int) string {
	value := strings.TrimSpace(message)
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
