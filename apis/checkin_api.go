package apis

import (
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type CheckinApi struct{}

func (a CheckinApi) Settings(c *gin.Context) {
	response.Ok(services.CheckinServiceApp.Settings(), c)
}

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

func (a CheckinApi) Status(c *gin.Context) {
	status, err := services.CheckinServiceApp.Status(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(status, c)
}

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
