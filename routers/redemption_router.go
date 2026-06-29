package routers

import "github.com/gin-gonic/gin"

type RedemptionRouter struct{}

func (r RedemptionRouter) InitRedemptionRouter(router *gin.RouterGroup) {
	group := router.Group("redemption")
	{
		group.GET("/list", redemptionApi.List)
		group.GET("/stats", redemptionApi.Stats)
		group.POST("/", redemptionApi.Create)
		group.POST("/batch", redemptionApi.BatchCreate)
		group.PUT("/", redemptionApi.Update)
		group.DELETE("/:id", redemptionApi.Delete)
		group.POST("/redeem", redemptionApi.Redeem)
	}

	card := router.Group("card")
	{
		card.GET("/list", redemptionApi.List)
		card.GET("/stats", redemptionApi.Stats)
		card.POST("/", redemptionApi.Create)
		card.POST("/batch", redemptionApi.BatchCreate)
		card.PUT("/", redemptionApi.Update)
		card.DELETE("/:id", redemptionApi.Delete)
		card.POST("/redeem", redemptionApi.Redeem)
	}
}
