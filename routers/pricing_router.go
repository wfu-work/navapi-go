package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type PricingRouter struct{}

func (r PricingRouter) InitPricingRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("pricing", pricingApi.Public)

	group := privateGroup.Group("pricing")
	{
		group.GET("/list", pricingApi.List)
		group.POST("/", middlewares.AdminOnly(), pricingApi.Upsert)
		group.PUT("/", middlewares.AdminOnly(), pricingApi.Upsert)
		group.DELETE("/:id", middlewares.AdminOnly(), pricingApi.Delete)
	}
}
