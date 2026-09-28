package llm

import "testing"

func TestCustomAndUpstreamMediaProtocolsCoexist(t *testing.T) {
	for _, tc := range []struct {
		protocol string
		endpoint string
		edit     bool
		video    bool
		stream   bool
	}{
		{AdapterOpenAIVideo, EndpointVideoGenerations, false, true, false},
		{AdapterImageEditsJSON, EndpointImageEdits, true, false, false},
		{AdapterOpenAIImageGenerations, EndpointImageGenerations, true, false, true},
		{AdapterOpenRouterImages, EndpointImages, true, false, true},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			if !IsImplementedAdapter(tc.protocol) {
				t.Fatal("protocol missing from shared contract")
			}
			if got := DefaultEndpointForAdapter(tc.protocol); got != tc.endpoint {
				t.Fatalf("endpoint = %q, want %q", got, tc.endpoint)
			}
			if got := IsImageEditAdapter(tc.protocol); got != tc.edit {
				t.Fatalf("image edit = %v, want %v", got, tc.edit)
			}
			if got := IsVideoGenerationAdapter(tc.protocol); got != tc.video {
				t.Fatalf("video = %v, want %v", got, tc.video)
			}
			if got := SupportsStreamingAdapter(tc.protocol); got != tc.stream {
				t.Fatalf("stream = %v, want %v", got, tc.stream)
			}
		})
	}
}

func TestCustomMediaFieldsPreserveTemporaryRequestContract(t *testing.T) {
	progress, taskID := 0, ""
	in := GenerateInput{
		Ephemeral:     true,
		OnProgress:    func(p int) { progress = p },
		OnTaskStarted: func(id string) { taskID = id },
		Messages:      []Message{{Parts: []ContentPart{{Kind: ContentPartAudio, URL: "https://example.invalid/audio"}}}},
	}
	in.OnProgress(50)
	in.OnTaskStarted("video-task")
	if !in.Ephemeral || progress != 50 || taskID != "video-task" {
		t.Fatal("custom callbacks and temporary request flag must coexist")
	}
	out := GenerateOutput{TextToolCallsStripped: true, GeneratedVideos: []GeneratedVideo{{FallbackURL: "https://example.invalid/video"}}}
	if !out.TextToolCallsStripped || out.GeneratedVideos[0].FallbackURL == "" {
		t.Fatal("custom output metadata lost")
	}
}
