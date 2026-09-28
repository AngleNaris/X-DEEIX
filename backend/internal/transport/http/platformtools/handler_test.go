package platformtools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func TestApprovalEndpointsRequireIdentifierBeforeServiceAccess(t *testing.T) {
	h := NewHandler(nil)
	for name, handle := range map[string]func(*gin.Context){"get": h.GetApproval, "approve": h.Approve, "reject": h.Reject} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			handle(ctx)
			var body response.Envelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusBadRequest || body.ErrorCode != "request.required" || body.ErrorMsg == "" {
				t.Fatalf("invalid missing-identifier contract: status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
