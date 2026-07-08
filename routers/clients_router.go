package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type ClientsRouter struct{}

func (r ClientsRouter) InitClientsRouter(router *gin.RouterGroup) {
	clients := router.Group("clients", middlewares.AdminOnly())
	{
		register := clients.Group("register")
		{
			register.GET("/settings", optionApi.RegisterSettings)
			register.PUT("/settings", optionApi.SetRegisterSettings)
		}

		invitations := clients.Group("invitations")
		{
			invitations.GET("/settings", invitationApi.Settings)
			invitations.PUT("/settings", invitationApi.SetSettings)
			invitations.GET("/codes", invitationApi.Codes)
			invitations.POST("/code", invitationApi.SaveCode)
			invitations.PUT("/code", invitationApi.SaveCode)
			invitations.GET("/code/:id", invitationApi.GetCode)
			invitations.DELETE("/code/:id", invitationApi.DeleteCode)
			invitations.GET("/relations", invitationApi.Relations)
			invitations.GET("/stats", invitationApi.Stats)
		}

		checkin := clients.Group("checkin")
		{
			checkin.GET("/settings", checkinApi.Settings)
			checkin.PUT("/settings", checkinApi.SetSettings)
			checkin.GET("/list", checkinApi.List)
		}

		users := clients.Group("users")
		{
			users.GET("/list", clientUserApi.List)
		}
	}
}
