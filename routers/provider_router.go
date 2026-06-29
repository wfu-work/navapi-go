package routers

import "github.com/gin-gonic/gin"

type ProviderRouter struct{}

func (r ProviderRouter) InitProviderRouter(router *gin.RouterGroup) {
	group := router.Group("provider")
	{
		group.GET("/list", providerApi.List)
		group.GET("/:id/key", providerApi.Key)
		group.PUT("/:id/key", providerApi.SetKey)
		group.POST("/:id/channel", providerApi.CreateChannel)
		group.GET("/:id", providerApi.Get)
		group.POST("/", providerApi.Save)
		group.PUT("/", providerApi.Save)
		group.DELETE("/:id", providerApi.Delete)
	}
}
