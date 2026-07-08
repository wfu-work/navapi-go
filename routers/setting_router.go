package routers

import (
	navapiMiddlewares "navapi-go/middlewares"

	"github.com/gin-gonic/gin"
	commonMiddlewares "github.com/wfu-work/nav-common-go-lib/middlewares"
)

type SettingRouter struct{}

func (s *SettingRouter) InitSettingRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("contact/settings", settingApi.Contact)

	groupLogger := privateGroup.Group("settings", navapiMiddlewares.AdminOnly()).Use(commonMiddlewares.ApiLogger())
	group := privateGroup.Group("settings", navapiMiddlewares.AdminOnly())
	{
		group.GET("list", settingApi.List)
		group.GET("contact", settingApi.Contact)
	}
	{
		groupLogger.PUT("contact", settingApi.SaveContact)
		groupLogger.POST("", settingApi.Save)
		groupLogger.DELETE(":guid", settingApi.Delete)
	}
}
