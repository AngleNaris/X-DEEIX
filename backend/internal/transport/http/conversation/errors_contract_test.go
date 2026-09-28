package conversation

import (
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"net/http"
	"testing"
)

func TestTransportErrorsKeepStableCodes(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{errInvalidFileID, "file.invalid_id"},
		{errInvalidFileSignature, "file.invalid_signature"},
		{errInvalidConversationID, "conversation.invalid_id"},
		{errInvalidRunIDs, "request.invalid_run_ids"},
		{errInvalidTemporaryChatMessages, "request.invalid_temporary_chat_messages"},
		{errTemporaryChatContextTooLarge, "temporary_chat.context_too_large"},
	} {
		got := response.Describe(http.StatusBadRequest, fmt.Errorf("wrapped: %w", tc.err))
		if got.Code != tc.code || got.Message != tc.err.Error() {
			t.Fatalf("description = %+v, want %s / %s", got, tc.code, tc.err)
		}
	}
}
