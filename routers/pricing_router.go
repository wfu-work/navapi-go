package routers

import "github.com/gin-gonic/gin"

type PricingRouter struct{}

func (r PricingRouter) InitPricingRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("pricing", pricingApi.Public)

	group := privateGroup.Group("pricing")
	{
		group.GET("/list", pricingApi.List)
		group.POST("/", pricingApi.Upsert)
		group.PUT("/", pricingApi.Upsert)
		group.DELETE("/:id", pricingApi.Delete)
	}
}
