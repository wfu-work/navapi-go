package routers

import "github.com/gin-gonic/gin"

type MessageTemplateRouter struct{}

func (r MessageTemplateRouter) InitMessageTemplateRouter(router *gin.RouterGroup) {
	group := router.Group("templates")
	{
		group.GET("list", messageTemplateApi.List)
		group.GET(":identity", messageTemplateApi.Get)
		group.POST("", messageTemplateApi.Save)
		group.POST("preview", messageTemplateApi.Preview)
		group.DELETE(":guid", messageTemplateApi.Disable)
	}
}
