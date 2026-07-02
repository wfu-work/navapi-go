package routers

import "github.com/gin-gonic/gin"

type AnnouncementRouter struct{}

func (r AnnouncementRouter) InitAnnouncementRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("announcements/list", announcementApi.PublicList)
	publicGroup.GET("announcements/latest", announcementApi.PublicLatest)

	group := privateGroup.Group("announcement")
	{
		group.GET("/list", announcementApi.List)
		group.GET("/:id", announcementApi.Get)
		group.POST("/", announcementApi.Save)
		group.PUT("/", announcementApi.Save)
		group.PUT("/:id", announcementApi.Save)
		group.DELETE("/:id", announcementApi.Delete)
	}
}
