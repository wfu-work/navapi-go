package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type CheckinRouter struct{}

func (r CheckinRouter) InitCheckinRouter(router *gin.RouterGroup) {
	group := router.Group("checkin")
	{
		group.GET("/settings", checkinApi.Settings)
		group.PUT("/settings", middlewares.AdminOnly(), checkinApi.SetSettings)
		group.GET("/list", middlewares.AdminOnly(), checkinApi.List)
		group.GET("/self/list", checkinApi.Self)
		group.GET("/self/status", checkinApi.Status)
		group.POST("/self", checkinApi.Checkin)
	}
}
