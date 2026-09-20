package channel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPersonalChannelRoutesRemoved(t *testing.T) {
	router := gin.New()
	m := NewModule(&Handler{})
	m.RegisterRoutes(router.Group("/api/v1"))
	m.RegisterAdminRoutes(router.Group("/api/v1/admin"))
	for _, route := range router.Routes() {
		if strings.Contains(route.Path, "/user/") || strings.Contains(route.Path, "user-upstream-presets") {
			t.Fatalf("retired route still registered: %s", route.Path)
		}
	}
	for _, path := range []string{"/api/v1/user/upstreams", "/api/v1/user/models", "/api/v1/admin/llm/user-upstream-presets"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != http.StatusNotFound {
				t.Fatalf("%s %s returned %d", method, path, w.Code)
			}
		}
	}
}
