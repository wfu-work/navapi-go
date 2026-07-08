package apis

import (
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type MessageTemplateApi struct{}

func (a MessageTemplateApi) List(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := messageTemplateService.List(query, c.Query("channel"), c.Query("status"))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a MessageTemplateApi) Get(c *gin.Context) {
	item, err := messageTemplateService.Get(c.Param("identity"))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(item, c)
}

func (a MessageTemplateApi) Save(c *gin.Context) {
	var req services.SaveMessageTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	item, err := messageTemplateService.Save(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(item, c)
}

func (a MessageTemplateApi) Disable(c *gin.Context) {
	if err := messageTemplateService.Disable(c.Param("guid")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a MessageTemplateApi) Preview(c *gin.Context) {
	var req services.EmailTemplatePreviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result, err := emailService.PreviewTemplate(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}
