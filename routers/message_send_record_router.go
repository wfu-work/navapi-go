package routers

import "github.com/gin-gonic/gin"

type MessageSendRecordRouter struct{}

func (r MessageSendRecordRouter) InitMessageSendRecordRouter(router *gin.RouterGroup) {
	group := router.Group("send-records")
	{
		group.GET("list", messageSendRecordApi.List)
		group.GET(":guid", messageSendRecordApi.Get)
	}
}
