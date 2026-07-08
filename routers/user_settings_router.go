package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type UserSettingsRouter struct{}

func (r UserSettingsRouter) InitUserSettingsRouter(router *gin.RouterGroup) {
	group := router.Group("user-settings")
	{
		group.GET("/admin/:userGuid", middlewares.AdminOnly(), userSettingsApi.GetByUser)
		group.PUT("/admin/:userGuid/max-concurrency", middlewares.AdminOnly(), userSettingsApi.SetMaxConcurrency)
		group.GET("/self", userSettingsApi.Self)
		group.PUT("/self", userSettingsApi.SaveSelf)
	}
}
