package apis

import (
	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type UserSettingsApi struct{}

func (a UserSettingsApi) Self(c *gin.Context) {
	settings, err := services.UserSettingsServiceApp.Get(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(settings, c)
}

func (a UserSettingsApi) SaveSelf(c *gin.Context) {
	var req domains.UserSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	settings, err := services.UserSettingsServiceApp.SavePreferences(utils.GetUserGuid(c), &req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(settings, c)
}
