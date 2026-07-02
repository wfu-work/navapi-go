package apis

import (
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type CheckinApi struct{}

// Settings 签到设置
// @Summary 签到设置
// @Description 签到设置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.CheckinSettings,msg=string}
// @Router /checkin/settings [get]
func (a CheckinApi) Settings(c *gin.Context) {
	response.Ok(services.CheckinServiceApp.Settings(), c)
}

// SetSettings 设置签到配置
// @Summary 设置签到配置
// @Description 设置签到配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.CheckinSettings true "签到设置对象"
// @Success 200 {object} response.Response{data=services.CheckinSettings,msg=string}
// @Router /checkin/settings [put]
func (a CheckinApi) SetSettings(c *gin.Context) {
	var settings services.CheckinSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.CheckinServiceApp.SetSettings(settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(services.CheckinServiceApp.Settings(), c)
}

// Status 当前用户签到状态
// @Summary 当前用户签到状态
// @Description 当前用户签到状态
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /checkin/self/status [get]
func (a CheckinApi) Status(c *gin.Context) {
	status, err := services.CheckinServiceApp.Status(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(status, c)
}

// Checkin 用户签到
// @Summary 用户签到
// @Description 用户签到
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.CheckinRequest true "签到请求对象"
// @Success 200 {object} response.Response{data=domains.CheckinRecord,msg=string}
// @Router /checkin/self [post]
func (a CheckinApi) Checkin(c *gin.Context) {
	var req services.CheckinRequest
	_ = c.ShouldBindJSON(&req)
	record, err := services.CheckinServiceApp.Checkin(utils.GetUserGuid(c), req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(record, c)
}

// List 签到记录列表
// @Summary 签到记录列表
// @Description 签到记录列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /checkin/list [get]
func (a CheckinApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.CheckinServiceApp.List("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Self 当前用户签到记录
// @Summary 当前用户签到记录
// @Description 当前用户签到记录
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /checkin/self/list [get]
func (a CheckinApi) Self(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.CheckinServiceApp.List(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}
