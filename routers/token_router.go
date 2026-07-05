package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type TokenRouter struct{}

func (r TokenRouter) InitTokenRouter(router *gin.RouterGroup) {
	router.GET("usage/token", tokenApi.Usage)

	group := router.Group("token")
	{
		group.GET("/self/list", tokenApi.SelfList)
		group.GET("/self/:id", tokenApi.SelfGet)
		group.POST("/self", tokenApi.CreateSelf)
		group.PUT("/self", tokenApi.UpdateSelf)
		group.DELETE("/self/:id", tokenApi.DeleteSelf)
		group.POST("/self/:id/key", tokenApi.KeySelf)

		group.GET("/list", middlewares.AdminOnly(), tokenApi.List)
		group.GET("/:id", middlewares.AdminOnly(), tokenApi.Get)
		group.POST("/", middlewares.AdminOnly(), tokenApi.Create)
		group.PUT("/", middlewares.AdminOnly(), tokenApi.Update)
		group.DELETE("/:id", middlewares.AdminOnly(), tokenApi.Delete)
		group.POST("/:id/key", middlewares.AdminOnly(), tokenApi.Key)
	}
}
