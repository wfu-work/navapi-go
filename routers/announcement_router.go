package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type AnnouncementRouter struct{}

func (r AnnouncementRouter) InitAnnouncementRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("announcements/list", announcementApi.PublicList)
	publicGroup.GET("announcements/latest", announcementApi.PublicLatest)

	group := privateGroup.Group("announcement")
	{
		group.GET("/client/list", announcementApi.ClientList)
		group.GET("/list", middlewares.AdminOnly(), announcementApi.List)
		group.GET("/:id", middlewares.AdminOnly(), announcementApi.Get)
		group.POST("/", middlewares.AdminOnly(), announcementApi.Save)
		group.PUT("/", middlewares.AdminOnly(), announcementApi.Save)
		group.PUT("/:id", middlewares.AdminOnly(), announcementApi.Save)
		group.DELETE("/:id", middlewares.AdminOnly(), announcementApi.Delete)
	}
}
