package routers

import "github.com/gin-gonic/gin"

type LogRouter struct{}

func (r LogRouter) InitLogRouter(router *gin.RouterGroup) {
	router.GET("data/list", logApi.Data)
	router.GET("data/self/list", logApi.SelfData)

	group := router.Group("log")
	{
		group.GET("/list", logApi.List)
		group.GET("/stat", logApi.Stats)
		group.GET("/self/list", logApi.Self)
		group.GET("/self/stat", logApi.SelfStats)
	}
}
