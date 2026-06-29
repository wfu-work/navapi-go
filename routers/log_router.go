package routers

import "github.com/gin-gonic/gin"

type LogRouter struct{}

func (r LogRouter) InitLogRouter(router *gin.RouterGroup) {
	router.GET("data/list", logApi.Data)
	router.GET("data/self/list", logApi.SelfData)
	router.GET("usage/summary", logApi.UsageSummary)
	router.GET("usage/self/summary", logApi.SelfUsageSummary)

	group := router.Group("log")
	{
		group.GET("/list", logApi.List)
		group.GET("/stat", logApi.Stats)
		group.GET("/summary", logApi.UsageSummary)
		group.GET("/self/list", logApi.Self)
		group.GET("/self/stat", logApi.SelfStats)
		group.GET("/self/summary", logApi.SelfUsageSummary)
	}
}
