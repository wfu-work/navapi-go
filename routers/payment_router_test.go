package routers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/configs"
)

func TestPaymentConfirmRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	publicGroup := engine.Group("/api")
	privateGroup := engine.Group("/api")
	privateGroup.Use(func(c *gin.Context) {
		c.Set("claims", &configs.CustomClaims{
			BaseClaims: configs.BaseClaims{Username: "demo", RoleCodes: "USER"},
		})
		c.Next()
	})

	PaymentRouter{}.InitPaymentRouter(privateGroup, publicGroup)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/payment/confirm", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for non-admin payment confirm", recorder.Code)
	}
}
