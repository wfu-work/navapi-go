package routers

import "github.com/gin-gonic/gin"

type ModelRouter struct{}

func (r ModelRouter) InitModelRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("models", relayApi.Models)
	publicGroup.GET("vendors", modelApi.PublicVendors)

	group := privateGroup.Group("models")
	{
		group.GET("/list", modelApi.List)
		group.POST("/", modelApi.Upsert)
		group.PUT("/", modelApi.Upsert)
		group.DELETE("/:id", modelApi.Delete)
	}

	vendors := privateGroup.Group("vendors")
	{
		vendors.GET("/list", modelApi.Vendors)
		vendors.POST("/", modelApi.UpsertVendor)
		vendors.PUT("/", modelApi.UpsertVendor)
		vendors.DELETE("/:id", modelApi.DeleteVendor)
	}
}
