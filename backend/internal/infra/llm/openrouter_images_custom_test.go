package llm

import (
	"strings"
	"testing"
)

func TestOpenRouterImageURLReferencesPreservedAndRedacted(t *testing.T) {
	const url = "https://example.com/private-image.png?token=sensitive"
	payload, debug, err := buildOpenRouterImageRequestBody("test", GenerateInput{Messages: []Message{{Role: "user", Content: "edit", Parts: []ContentPart{{Kind: ContentPartText, Text: "edit"}, {Kind: ContentPartImage, URL: url}}}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	refs, ok := payload["input_references"].([]map[string]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("references: %#v", payload)
	}
	if refs[0]["image_url"].(map[string]any)["url"] != url {
		t.Fatalf("URL lost: %#v", refs)
	}
	if strings.Contains(string(debug), "sensitive") || strings.Contains(string(debug), "example.com") {
		t.Fatalf("reference leaked: %s", debug)
	}
}
