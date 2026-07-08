package apis

import (
	"navapi-go/domains"
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type InvitationApi struct{}

// Settings 邀请设置
// @Summary 邀请设置
// @Description 邀请设置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.InviteSettings,msg=string}
// @Router /invitation/settings [get]
func (a InvitationApi) Settings(c *gin.Context) {
	response.Ok(invitationService.Settings(), c)
}

// SetSettings 设置邀请配置
// @Summary 设置邀请配置
// @Description 设置邀请配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.InviteSettings true "邀请设置对象"
// @Success 200 {object} response.Response{data=services.InviteSettings,msg=string}
// @Router /invitation/settings [put]
func (a InvitationApi) SetSettings(c *gin.Context) {
	var settings services.InviteSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := invitationService.SetSettings(settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(invitationService.Settings(), c)
}

// MyCode 当前用户邀请码
// @Summary 当前用户邀请码
// @Description 当前用户邀请码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=domains.InvitationCode,msg=string}
// @Router /invitation/self/code [get]
func (a InvitationApi) MyCode(c *gin.Context) {
	code, err := invitationService.EnsureSelfCode(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(code, c)
}

// MyCodes 当前用户邀请码列表
// @Summary 当前用户邀请码列表
// @Description 当前用户邀请码列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /invitation/self/codes [get]
func (a InvitationApi) MyCodes(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := invitationService.ListCodes(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Codes 邀请码列表
// @Summary 邀请码列表
// @Description 邀请码列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /invitation/codes [get]
func (a InvitationApi) Codes(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := invitationService.ListCodes("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// SaveCode 保存邀请码
// @Summary 保存邀请码
// @Description 保存邀请码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.InvitationCode true "邀请码对象"
// @Success 200 {object} response.Response{data=domains.InvitationCode,msg=string}
// @Router /invitation/code [post]
// @Router /invitation/code [put]
func (a InvitationApi) SaveCode(c *gin.Context) {
	var code domains.InvitationCode
	if err := c.ShouldBindJSON(&code); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := invitationService.SaveCode(&code); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(code, c)
}

// GetCode 邀请码详情
// @Summary 邀请码详情
// @Description 邀请码详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.InvitationCode,msg=string}
// @Router /invitation/code/{id} [get]
func (a InvitationApi) GetCode(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	code, err := invitationService.GetCode(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(code, c)
}

// DeleteCode 删除邀请码
// @Summary 删除邀请码
// @Description 删除邀请码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /invitation/code/{id} [delete]
func (a InvitationApi) DeleteCode(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := invitationService.DeleteCode(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Accept 接受邀请
// @Summary 接受邀请
// @Description 接受邀请
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.AcceptInviteRequest true "接受邀请请求"
// @Success 200 {object} response.Response{data=domains.InvitationRelation,msg=string}
// @Router /invitation/accept [post]
func (a InvitationApi) Accept(c *gin.Context) {
	var req services.AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	relation, err := invitationService.AcceptInvite(utils.GetUserGuid(c), req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(relation, c)
}

// Relations 邀请关系列表
// @Summary 邀请关系列表
// @Description 邀请关系列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /invitation/relations [get]
func (a InvitationApi) Relations(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := invitationService.ListRelations("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// MyRelations 当前用户邀请关系
// @Summary 当前用户邀请关系
// @Description 当前用户邀请关系
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /invitation/self/relations [get]
func (a InvitationApi) MyRelations(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := invitationService.ListRelations(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Stats 邀请统计
// @Summary 邀请统计
// @Description 邀请统计
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /invitation/stats [get]
func (a InvitationApi) Stats(c *gin.Context) {
	stats, err := invitationService.Stats("")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

// MyStats 当前用户邀请统计
// @Summary 当前用户邀请统计
// @Description 当前用户邀请统计
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /invitation/self/stats [get]
func (a InvitationApi) MyStats(c *gin.Context) {
	stats, err := invitationService.Stats(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}
