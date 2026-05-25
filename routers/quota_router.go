package routers

import "github.com/gin-gonic/gin"

type QuotaRouter struct{}

func (r QuotaRouter) InitQuotaRouter(router *gin.RouterGroup) {
	group := router.Group("quota")
	{
		group.GET("/list", quotaApi.List)
		group.GET("/self", quotaApi.Self)
		group.PUT("/", quotaApi.Update)
	}
}
