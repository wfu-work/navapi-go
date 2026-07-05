package routers

import "github.com/gin-gonic/gin"

type UserSettingsRouter struct{}

func (r UserSettingsRouter) InitUserSettingsRouter(router *gin.RouterGroup) {
	group := router.Group("user-settings")
	{
		group.GET("/self", userSettingsApi.Self)
		group.PUT("/self", userSettingsApi.SaveSelf)
	}
}
