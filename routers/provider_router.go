package routers

import "github.com/gin-gonic/gin"

type ProviderRouter struct{}

func (r ProviderRouter) InitProviderRouter(router *gin.RouterGroup) {
	group := router.Group("provider")
	{
		group.GET("/list", providerApi.List)
		group.POST("/test", providerApi.Test)
		group.GET("/:guid/key", providerApi.Key)
		group.PUT("/:guid/key", providerApi.SetKey)
		group.GET("/:guid", providerApi.Get)
		group.POST("/", providerApi.Save)
		group.PUT("/", providerApi.Save)
		group.DELETE("/:guid", providerApi.Delete)
	}
}
