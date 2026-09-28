package conversation

import (
	"encoding/json"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/gin-gonic/gin/binding"
	"strings"
	"testing"
)

func TestFileBatchRequestsEnforceBounds(t *testing.T) {
	valid := make([]string, 100)
	for i := range valid {
		valid[i] = "file_id"
	}
	cases := []struct {
		name string
		ids  []string
		ok   bool
	}{
		{"missing", nil, false}, {"empty", []string{}, false},
		{"empty member", []string{""}, false},
		{"long id", []string{strings.Repeat("x", 65)}, false},
		{"maximum", valid, true},
		{"too many", append(append([]string{}, valid...), "extra"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, req := range []any{SubmitFileEmbeddingsRequest{FileIDs: tc.ids}, GetFileProcessingStatusesRequest{FileIDs: tc.ids}} {
				err := binding.Validator.ValidateStruct(req)
				if (err == nil) != tc.ok {
					t.Fatalf("%T validation = %v, valid=%v", req, err, tc.ok)
				}
			}
		})
	}
}

func TestEmbeddingSubmissionResponsePreservesReasons(t *testing.T) {
	got := toFileEmbeddingSubmissionResponse(appembedding.TargetedSubmissionResult{
		SubmittedFileIDs: []string{"accepted"},
		Skipped:          []appembedding.TargetedFileSkip{{FileID: "skipped", Reason: "already_queued"}},
	})
	if len(got.SubmittedFileIDs) != 1 || got.SubmittedFileIDs[0] != "accepted" || len(got.Skipped) != 1 || got.Skipped[0].Reason != "already_queued" || got.Skipped[0].FileID != "skipped" {
		t.Fatalf("lost submission result: %+v", got)
	}
	empty := toFileEmbeddingSubmissionResponse(appembedding.TargetedSubmissionResult{SubmittedFileIDs: []string{}})
	raw, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"skipped":[]`) {
		t.Fatalf("empty skipped should be an array: %s", raw)
	}
}
