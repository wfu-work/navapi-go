package routers

import "github.com/gin-gonic/gin"

type TokenRouter struct{}

func (r TokenRouter) InitTokenRouter(router *gin.RouterGroup) {
	router.GET("usage/token", tokenApi.Usage)

	group := router.Group("token")
	{
		group.GET("/list", tokenApi.List)
		group.GET("/:id", tokenApi.Get)
		group.POST("/", tokenApi.Create)
		group.PUT("/", tokenApi.Update)
		group.DELETE("/:id", tokenApi.Delete)
		group.POST("/:id/key", tokenApi.Key)
	}
}
