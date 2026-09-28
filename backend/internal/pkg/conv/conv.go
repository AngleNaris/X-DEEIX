package conv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GetIntFromAny converts supported numeric values and decimal strings to int.
// Invalid inputs return zero, retaining the custom options conversion contract.
func GetIntFromAny(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0
		}
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// NormalizePublicID 移除 UUID 连字符并去除首尾空白。
func NormalizePublicID(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "")
}

// GetStringFromAny 将任意类型转换为字符串。
func GetStringFromAny(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
