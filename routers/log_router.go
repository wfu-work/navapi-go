package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type LogRouter struct{}

func (r LogRouter) InitLogRouter(router *gin.RouterGroup) {
	router.GET("data/self/list", logApi.SelfData)
	router.GET("data/list", middlewares.AdminOnly(), logApi.Data)

	group := router.Group("usage")
	{
		group.GET("/self/list", logApi.Self)
		group.GET("/self/stat", logApi.SelfStats)
		group.GET("/self/summary", logApi.SelfUsageSummary)
		group.GET("/list", middlewares.AdminOnly(), logApi.List)
		group.GET("/stat", middlewares.AdminOnly(), logApi.Stats)
		group.GET("/summary", middlewares.AdminOnly(), logApi.UsageSummary)
	}
}
