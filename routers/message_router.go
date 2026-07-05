package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type MessageRouter struct{}

func (r MessageRouter) InitMessageRouter(router *gin.RouterGroup) {
	group := router.Group("messages", middlewares.AdminOnly())
	MessageEmailConfigRouter{}.InitMessageEmailConfigRouter(group)
	MessageTemplateRouter{}.InitMessageTemplateRouter(group)
	MessageSendRecordRouter{}.InitMessageSendRecordRouter(group)
}
