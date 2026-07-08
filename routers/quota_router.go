package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type QuotaRouter struct{}

func (r QuotaRouter) InitQuotaRouter(router *gin.RouterGroup) {
	group := router.Group("balance")
	{
		group.GET("/list", middlewares.AdminOnly(), quotaApi.List)
		group.GET("/self", quotaApi.Self)
		group.PUT("/", middlewares.AdminOnly(), quotaApi.Update)
	}
}
