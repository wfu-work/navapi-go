package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type GatewayRouter struct{}

func (r GatewayRouter) InitGatewayRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("service/status", gatewayApi.PublicStatus)

	group := privateGroup.Group("gateway", middlewares.AdminOnly())
	{
		group.GET("/health", gatewayApi.Health)
	}
}
