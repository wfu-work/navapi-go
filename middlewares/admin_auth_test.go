package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/configs"
)

func TestAdminOnlyBlocksNonAdminUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin-only", func(c *gin.Context) {
		c.Set("claims", &configs.CustomClaims{
			BaseClaims: configs.BaseClaims{Username: "demo", RoleCodes: "SUPER_ADMIN"},
		})
	}, AdminOnly(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin-only", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestAdminOnlyAllowsAdminUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin-only", func(c *gin.Context) {
		c.Set("claims", &configs.CustomClaims{
			BaseClaims: configs.BaseClaims{Username: "admin", RoleCodes: "USER"},
		})
	}, AdminOnly(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin-only", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}
