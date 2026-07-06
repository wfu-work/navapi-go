package apis

import (
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type MessageEmailConfigApi struct{}

func (a MessageEmailConfigApi) List(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.MessageEmailConfigServiceApp.List(query, c.Query("status"))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a MessageEmailConfigApi) Save(c *gin.Context) {
	var req services.SaveMessageEmailConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	item, err := services.MessageEmailConfigServiceApp.Save(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(item, c)
}

func (a MessageEmailConfigApi) SetDefault(c *gin.Context) {
	if err := services.MessageEmailConfigServiceApp.SetDefault(c.Param("guid")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a MessageEmailConfigApi) DebugSend(c *gin.Context) {
	var req services.DebugEmailConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	req.ConfigGuid = c.Param("guid")
	result, err := services.EmailServiceApp.DebugEmailConfig(req)
	if err != nil {
		if result != nil {
			response.FailWithDetailed(result, err.Error(), c)
			return
		}
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a MessageEmailConfigApi) Disable(c *gin.Context) {
	if err := services.MessageEmailConfigServiceApp.Disable(c.Param("guid")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
