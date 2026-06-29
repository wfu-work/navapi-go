package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type InvitationApi struct{}

func (a InvitationApi) Settings(c *gin.Context) {
	response.Ok(services.InvitationServiceApp.Settings(), c)
}

func (a InvitationApi) SetSettings(c *gin.Context) {
	var settings services.InviteSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.InvitationServiceApp.SetSettings(settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(services.InvitationServiceApp.Settings(), c)
}

func (a InvitationApi) MyCode(c *gin.Context) {
	code, err := services.InvitationServiceApp.EnsureSelfCode(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(code, c)
}

func (a InvitationApi) MyCodes(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.InvitationServiceApp.ListCodes(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a InvitationApi) Codes(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.InvitationServiceApp.ListCodes("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a InvitationApi) SaveCode(c *gin.Context) {
	var code domains.InvitationCode
	if err := c.ShouldBindJSON(&code); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.InvitationServiceApp.SaveCode(&code); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(code, c)
}

func (a InvitationApi) DeleteCode(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.InvitationServiceApp.DeleteCode(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a InvitationApi) Accept(c *gin.Context) {
	var req services.AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	relation, err := services.InvitationServiceApp.AcceptInvite(utils.GetUserGuid(c), req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(relation, c)
}

func (a InvitationApi) Relations(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.InvitationServiceApp.ListRelations("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a InvitationApi) MyRelations(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.InvitationServiceApp.ListRelations(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a InvitationApi) Stats(c *gin.Context) {
	stats, err := services.InvitationServiceApp.Stats("")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

func (a InvitationApi) MyStats(c *gin.Context) {
	stats, err := services.InvitationServiceApp.Stats(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}
