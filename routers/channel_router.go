package routers

import "github.com/gin-gonic/gin"

type ChannelRouter struct{}

func (r ChannelRouter) InitChannelRouter(router *gin.RouterGroup) {
	group := router.Group("channel")
	{
		group.GET("/list", channelApi.List)
		group.GET("/models", channelApi.Models)
		group.GET("/health_logs", channelApi.HealthLogs)
		group.GET("/test/:id", channelApi.Test)
		group.POST("/fetch_models", channelApi.FetchModels)
		group.POST("/batch", channelApi.Batch)
		group.POST("/tag/enabled", channelApi.EnableByTag)
		group.POST("/tag/disabled", channelApi.DisableByTag)
		group.GET("/:id/key", channelApi.Key)
		group.POST("/:id/key", channelApi.Key)
		group.PUT("/:id/key", channelApi.SetKey)
		group.PUT("/:id/upstream", channelApi.UpdateUpstream)
		group.GET("/:id/model_mapping", channelApi.GetModelMapping)
		group.PUT("/:id/model_mapping", channelApi.SetModelMapping)
		group.GET("/:id/health_logs", channelApi.ChannelHealthLogs)
		group.GET("/:id", channelApi.Get)
		group.POST("/", channelApi.Create)
		group.PUT("/", channelApi.Update)
		group.DELETE("/:id", channelApi.Delete)
	}
}
