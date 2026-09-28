package conversation

import "testing"

func TestIsFileProcessingIncludesActiveStagesAndExcludesEmpty(t *testing.T) {
	for _, status := range []string{"uploaded", "queued", "extracting", "embedding"} {
		if !IsFileProcessing(FileObject{ProcessingStatus: status}) {
			t.Errorf("active stage %s was omitted", status)
		}
	}
	for _, file := range []FileObject{
		{ProcessingStatus: "ready", ExtractStatus: "processing"},
		{ProcessingStatus: "ready", EmbedStatus: "queued"},
		{ProcessingStatus: "ready", EmbedStatus: "processing"},
	} {
		if !IsFileProcessing(file) {
			t.Errorf("active subprocess omitted: %+v", file)
		}
	}
	for _, status := range []string{"ready", "failed", "empty"} {
		file := FileObject{ProcessingStatus: status, ExtractStatus: status, EmbedStatus: status}
		if IsFileProcessing(file) {
			t.Errorf("terminal file classified active: %+v", file)
		}
	}
}
