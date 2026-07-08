package apis

import (
	"strings"

	"navapi-go/domains"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type UserSettingsApi struct{}

type userMaxConcurrencyRequest struct {
	MaxConcurrency int `json:"maxConcurrency" binding:"required"`
}

// GetByUser 管理端按用户 GUID 查看配置。Get 会自动补齐默认配置，详情页可以稳定展示。
func (a UserSettingsApi) GetByUser(c *gin.Context) {
	userGuid := strings.TrimSpace(c.Param("userGuid"))
	if userGuid == "" {
		response.FailWithMessage("user guid is required", c)
		return
	}
	settings, err := userSettingsService.Get(userGuid)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(settings, c)
}

// SetMaxConcurrency 管理端只更新用户最大并发，避免误覆盖用户提醒偏好。
func (a UserSettingsApi) SetMaxConcurrency(c *gin.Context) {
	userGuid := strings.TrimSpace(c.Param("userGuid"))
	if userGuid == "" {
		response.FailWithMessage("user guid is required", c)
		return
	}
	var req userMaxConcurrencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	settings, err := userSettingsService.SetMaxConcurrency(userGuid, req.MaxConcurrency)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(settings, c)
}

func (a UserSettingsApi) Self(c *gin.Context) {
	settings, err := userSettingsService.Get(utils.GetUserGuid(c))
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
	settings, err := userSettingsService.SavePreferences(utils.GetUserGuid(c), &req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(settings, c)
}
