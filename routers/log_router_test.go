package routers

import (
	"testing"

	"github.com/gin-gonic/gin"
	commonRouters "github.com/wfu-work/nav-common-go-lib/routers"
)

func TestBusinessRoutesDoNotConflictWithSystemRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	publicGroup := engine.Group("/api")
	privateGroup := engine.Group("/api")

	systemRouter := commonRouters.SysRouterGroupApp
	systemRouter.InitCompanyRouter(privateGroup)
	systemRouter.InitUserRouter(privateGroup)
	systemRouter.InitRoleRouter(privateGroup)
	systemRouter.InitLogRouter(privateGroup)
	systemRouter.InitOsRouter(privateGroup)
	systemRouter.InitConfigRouter(privateGroup)
	systemRouter.InitLoginRouter(privateGroup, publicGroup)
	systemRouter.InitLoginLogRouter(privateGroup)
	systemRouter.InitFileRouter(privateGroup)
	systemRouter.InitSecretRouter(publicGroup)
	systemRouter.InitRegisterRouter(publicGroup)

	RouterGroupApp.InitRouters(publicGroup, privateGroup)
}
