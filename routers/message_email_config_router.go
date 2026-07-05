package routers

import "github.com/gin-gonic/gin"

type MessageEmailConfigRouter struct{}

func (r MessageEmailConfigRouter) InitMessageEmailConfigRouter(router *gin.RouterGroup) {
	group := router.Group("email-configs")
	{
		group.GET("list", messageEmailConfigApi.List)
		group.POST("", messageEmailConfigApi.Save)
		group.POST(":guid/default", messageEmailConfigApi.SetDefault)
		group.POST(":guid/debug-send", messageEmailConfigApi.DebugSend)
		group.DELETE(":guid", messageEmailConfigApi.Disable)
	}
}
