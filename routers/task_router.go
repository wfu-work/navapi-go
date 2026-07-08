package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type TaskRouter struct{}

func (r TaskRouter) InitTaskRouter(router *gin.RouterGroup) {
	group := router.Group("task")
	{
		group.GET("/list", middlewares.AdminOnly(), taskApi.List)
		group.POST("/", middlewares.AdminOnly(), taskApi.Create)
		group.PUT("/", middlewares.AdminOnly(), taskApi.Update)
		group.GET("/self/list", taskApi.Self)
		group.POST("/self", taskApi.CreateSelf)
		group.PUT("/self", taskApi.UpdateSelf)
		group.GET("/self/:task_id", taskApi.GetSelf)
		group.DELETE("/self/:task_id", taskApi.DeleteSelf)
		group.GET("/:task_id", middlewares.AdminOnly(), taskApi.Get)
		group.DELETE("/:task_id", middlewares.AdminOnly(), taskApi.Delete)
	}
}
