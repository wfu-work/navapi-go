package routers

import "github.com/gin-gonic/gin"

type OptionRouter struct{}

func (r OptionRouter) InitOptionRouter(router *gin.RouterGroup) {
	group := router.Group("option")
	{
		group.GET("/list", optionApi.All)
		group.PUT("/", optionApi.Set)
		group.DELETE("/:key", optionApi.Delete)
	}
}
