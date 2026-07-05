package routers

import "github.com/gin-gonic/gin"

type RegisterRouter struct{}

func (r RegisterRouter) InitRegisterRouter(publicGroup *gin.RouterGroup) {
	group := publicGroup.Group("register")
	{
		group.POST("code", registerApi.SendCode)
		group.POST("client/user", registerApi.RegisterClient)
	}
}
