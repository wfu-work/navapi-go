package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type InvitationRouter struct{}

func (r InvitationRouter) InitInvitationRouter(router *gin.RouterGroup) {
	group := router.Group("invitation")
	{
		group.GET("/settings", middlewares.AdminOnly(), invitationApi.Settings)
		group.PUT("/settings", middlewares.AdminOnly(), invitationApi.SetSettings)
		group.GET("/codes", middlewares.AdminOnly(), invitationApi.Codes)
		group.POST("/code", middlewares.AdminOnly(), invitationApi.SaveCode)
		group.PUT("/code", middlewares.AdminOnly(), invitationApi.SaveCode)
		group.GET("/code/:id", middlewares.AdminOnly(), invitationApi.GetCode)
		group.DELETE("/code/:id", middlewares.AdminOnly(), invitationApi.DeleteCode)
		group.GET("/relations", middlewares.AdminOnly(), invitationApi.Relations)
		group.GET("/stats", middlewares.AdminOnly(), invitationApi.Stats)
		group.GET("/self/code", invitationApi.MyCode)
		group.GET("/self/codes", invitationApi.MyCodes)
		group.GET("/self/relations", invitationApi.MyRelations)
		group.GET("/self/stats", invitationApi.MyStats)
		group.POST("/accept", invitationApi.Accept)
	}
}
