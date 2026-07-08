package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type SystemMonitorRouter struct{}

func (r *SystemMonitorRouter) InitSystemMonitorRouter(router *gin.RouterGroup) {
	group := router.Group("system/monitor", middlewares.AdminOnly())
	{
		group.GET("runtime", systemMonitorApi.Runtime)
	}
}
