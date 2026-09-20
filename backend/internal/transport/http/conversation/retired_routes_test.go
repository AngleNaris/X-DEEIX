package conversation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConversationPromptRouteRemoved(t *testing.T) {
	router := gin.New()
	m := &Module{Handler: &Handler{}}
	m.RegisterRoutes(router.Group("/api/v1"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/existing/system-prompt", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("retired conversation prompt route returned %d", w.Code)
	}
}
