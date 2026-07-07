package middlewares

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/configs"
)

func TestScopedUserGuidReturnsAllScopeForAdminUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("claims", &configs.CustomClaims{
		BaseClaims: configs.BaseClaims{
			UserGuid: "admin-guid",
			Username: "admin",
		},
	})

	if userGuid := ScopedUserGuid(c); userGuid != "" {
		t.Fatalf("ScopedUserGuid() = %q, want empty admin scope", userGuid)
	}
}

func TestScopedUserGuidReturnsAllScopeForSuperAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("claims", &configs.CustomClaims{
		BaseClaims: configs.BaseClaims{
			UserGuid:  "admin-guid",
			Username:  "demo",
			RoleCodes: "USER;SUPER_ADMIN",
		},
	})

	if userGuid := ScopedUserGuid(c); userGuid != "" {
		t.Fatalf("ScopedUserGuid() = %q, want empty admin scope", userGuid)
	}
}

func TestScopedUserGuidReturnsCurrentUserForCommonUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("claims", configs.CustomClaims{
		BaseClaims: configs.BaseClaims{
			UserGuid: "user-guid",
			Username: "demo",
		},
	})

	if userGuid := ScopedUserGuid(c); userGuid != "user-guid" {
		t.Fatalf("ScopedUserGuid() = %q, want user-guid", userGuid)
	}
}
