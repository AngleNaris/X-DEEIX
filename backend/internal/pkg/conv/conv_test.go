package conv

import (
	"encoding/json"
	"testing"
)

func TestGetIntFromAny(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input any
		want  int
	}{
		{"int", 42, 42}, {"int64", int64(-42), -42},
		{"float", 42.9, 42}, {"negative float", -42.9, -42},
		{"json", json.Number("42"), 42}, {"json decimal", json.Number("42.5"), 0},
		{"string", " 42 ", 42}, {"invalid", "bad", 0},
		{"nil", nil, 0}, {"bool", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetIntFromAny(tc.input); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
