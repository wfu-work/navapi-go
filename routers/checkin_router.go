package routers

import "github.com/gin-gonic/gin"

type CheckinRouter struct{}

func (r CheckinRouter) InitCheckinRouter(router *gin.RouterGroup) {
	group := router.Group("checkin")
	{
		group.GET("/settings", checkinApi.Settings)
		group.PUT("/settings", checkinApi.SetSettings)
		group.GET("/list", checkinApi.List)
		group.GET("/self/list", checkinApi.Self)
		group.GET("/self/status", checkinApi.Status)
		group.POST("/self", checkinApi.Checkin)
	}
}
