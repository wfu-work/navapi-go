package routers

import "github.com/gin-gonic/gin"

type RedemptionRouter struct{}

func (r RedemptionRouter) InitRedemptionRouter(router *gin.RouterGroup) {
	group := router.Group("redemption")
	{
		group.GET("/list", redemptionApi.List)
		group.POST("/", redemptionApi.Create)
		group.PUT("/", redemptionApi.Update)
		group.DELETE("/:id", redemptionApi.Delete)
		group.POST("/redeem", redemptionApi.Redeem)
	}
}
