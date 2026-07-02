package routers

import "github.com/gin-gonic/gin"

type InvitationRouter struct{}

func (r InvitationRouter) InitInvitationRouter(router *gin.RouterGroup) {
	group := router.Group("invitation")
	{
		group.GET("/settings", invitationApi.Settings)
		group.PUT("/settings", invitationApi.SetSettings)
		group.GET("/codes", invitationApi.Codes)
		group.POST("/code", invitationApi.SaveCode)
		group.PUT("/code", invitationApi.SaveCode)
		group.GET("/code/:id", invitationApi.GetCode)
		group.DELETE("/code/:id", invitationApi.DeleteCode)
		group.GET("/relations", invitationApi.Relations)
		group.GET("/stats", invitationApi.Stats)
		group.GET("/self/code", invitationApi.MyCode)
		group.GET("/self/codes", invitationApi.MyCodes)
		group.GET("/self/relations", invitationApi.MyRelations)
		group.GET("/self/stats", invitationApi.MyStats)
		group.POST("/accept", invitationApi.Accept)
	}
}
