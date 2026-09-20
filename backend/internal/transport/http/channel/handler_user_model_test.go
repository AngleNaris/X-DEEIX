package channel

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/gin-gonic/gin"
)

func TestUserModelErrorProtocolValidation(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{appchannel.ErrInvalidAdapter, http.StatusBadRequest},
		{fmt.Errorf("model: %w", appchannel.ErrProtocolRequired), http.StatusBadRequest},
		{errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			userModelError(c, tc.err)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
