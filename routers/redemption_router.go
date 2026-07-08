package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type RedemptionRouter struct{}

func (r RedemptionRouter) InitRedemptionRouter(router *gin.RouterGroup) {
	group := router.Group("redemption")
	{
		group.GET("/list", middlewares.AdminOnly(), redemptionApi.List)
		group.GET("/stats", middlewares.AdminOnly(), redemptionApi.Stats)
		group.GET("/:id", middlewares.AdminOnly(), redemptionApi.Get)
		group.POST("/", middlewares.AdminOnly(), redemptionApi.Create)
		group.POST("/batch", middlewares.AdminOnly(), redemptionApi.BatchCreate)
		group.PUT("/", middlewares.AdminOnly(), redemptionApi.Update)
		group.DELETE("/:id", middlewares.AdminOnly(), redemptionApi.Delete)
		group.POST("/redeem", redemptionApi.Redeem)
	}

	card := router.Group("card")
	{
		card.GET("/list", middlewares.AdminOnly(), redemptionApi.List)
		card.GET("/stats", middlewares.AdminOnly(), redemptionApi.Stats)
		card.GET("/:id", middlewares.AdminOnly(), redemptionApi.Get)
		card.POST("/", middlewares.AdminOnly(), redemptionApi.Create)
		card.POST("/batch", middlewares.AdminOnly(), redemptionApi.BatchCreate)
		card.PUT("/", middlewares.AdminOnly(), redemptionApi.Update)
		card.DELETE("/:id", middlewares.AdminOnly(), redemptionApi.Delete)
		card.POST("/redeem", redemptionApi.Redeem)
	}
}
