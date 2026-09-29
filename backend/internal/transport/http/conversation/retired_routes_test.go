package conversation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestConversationPromptRouteRemoved(t *testing.T) {
	router := gin.New()
	m := &Module{Handler: &Handler{}}
	gate := middleware.NewFeatureGate(config.NewRuntime(config.Config{}))
	m.RegisterRoutes(router.Group("/api/v1"), gate)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/existing/system-prompt", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("retired conversation prompt route returned %d", w.Code)
	}
}
