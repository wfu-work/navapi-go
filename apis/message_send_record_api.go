package apis

import (
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type MessageSendRecordApi struct{}

func (a MessageSendRecordApi) List(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := messageSendRecordService.List(query, c.Query("sendStatus"), c.Query("templateCode"))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a MessageSendRecordApi) Get(c *gin.Context) {
	item, err := messageSendRecordService.Get(c.Param("guid"))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(item, c)
}
