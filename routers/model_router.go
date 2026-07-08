package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type ModelRouter struct{}

func (r ModelRouter) InitModelRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("models", relayApi.Models)
	publicGroup.GET("models/meta", modelApi.PublicList)
	publicGroup.GET("models/group-options", modelApi.PublicGroups)
	publicGroup.GET("vendors", modelApi.PublicVendors)

	group := privateGroup.Group("models")
	{
		group.GET("/list", modelApi.List)
		group.GET("/groups", modelApi.Groups)
		group.POST("/groups", middlewares.AdminOnly(), modelApi.UpsertGroup)
		group.PUT("/groups", middlewares.AdminOnly(), modelApi.UpsertGroup)
		group.DELETE("/groups/:guid", middlewares.AdminOnly(), modelApi.DeleteGroup)
		group.POST("/", middlewares.AdminOnly(), modelApi.Upsert)
		group.PUT("/", middlewares.AdminOnly(), modelApi.Upsert)
		group.DELETE("/:guid", middlewares.AdminOnly(), modelApi.Delete)
	}

	vendors := privateGroup.Group("vendors", middlewares.AdminOnly())
	{
		vendors.GET("/list", modelApi.Vendors)
		vendors.POST("/", modelApi.UpsertVendor)
		vendors.PUT("/", modelApi.UpsertVendor)
		vendors.DELETE("/:id", modelApi.DeleteVendor)
	}
}
