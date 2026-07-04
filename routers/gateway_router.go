package routers

import "github.com/gin-gonic/gin"

type GatewayRouter struct{}

func (r GatewayRouter) InitGatewayRouter(router *gin.RouterGroup) {
	group := router.Group("gateway")
	{
		group.GET("/health", gatewayApi.Health)
	}
}
