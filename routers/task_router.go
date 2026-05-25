package routers

import "github.com/gin-gonic/gin"

type TaskRouter struct{}

func (r TaskRouter) InitTaskRouter(router *gin.RouterGroup) {
	group := router.Group("task")
	{
		group.GET("/list", taskApi.List)
		group.POST("/", taskApi.Create)
		group.PUT("/", taskApi.Update)
		group.GET("/self/list", taskApi.Self)
		group.POST("/self", taskApi.CreateSelf)
		group.PUT("/self", taskApi.UpdateSelf)
		group.GET("/self/:task_id", taskApi.GetSelf)
		group.DELETE("/self/:task_id", taskApi.DeleteSelf)
		group.GET("/:task_id", taskApi.Get)
		group.DELETE("/:task_id", taskApi.Delete)
	}
}
