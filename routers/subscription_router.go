package routers

import "github.com/gin-gonic/gin"

type SubscriptionRouter struct{}

func (r SubscriptionRouter) InitSubscriptionRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("subscription/public/plans", subscriptionApi.PublicPlans)

	group := privateGroup.Group("subscription")
	{
		group.GET("/plans", subscriptionApi.Plans)
		group.POST("/plan", subscriptionApi.SavePlan)
		group.PUT("/plan", subscriptionApi.SavePlan)
		group.DELETE("/plan/:id", subscriptionApi.DeletePlan)
		group.GET("/list", subscriptionApi.List)
		group.GET("/self/list", subscriptionApi.Self)
		group.POST("/subscribe", subscriptionApi.Subscribe)
	}
}
