package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"navapi-go/domains"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRelayRouterRegistersRootAndAPIPrefixedV1(t *testing.T) {
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.Option{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previousDB
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	publicGroup := engine.Group("/api")
	privateGroup := engine.Group("/api")
	RouterGroupApp.InitRouters(publicGroup, privateGroup)

	for _, path := range []string{"/v1/models", "/api/v1/models"} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, req)
		if recorder.Code == http.StatusNotFound {
			t.Fatalf("%s returned 404, want relay route to be registered", path)
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401 without API key", path, recorder.Code)
		}
	}
}
