package apis

import (
	"strconv"

	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type AnnouncementApi struct{}

func (a AnnouncementApi) PublicList(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.AnnouncementServiceApp.List(query, true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a AnnouncementApi) PublicLatest(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	announcements, err := services.AnnouncementServiceApp.Latest(limit)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(announcements, c)
}

func (a AnnouncementApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.AnnouncementServiceApp.List(query, false)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a AnnouncementApi) Get(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	announcement, err := services.AnnouncementServiceApp.GetByID(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(announcement, c)
}

func (a AnnouncementApi) Save(c *gin.Context) {
	var announcement domains.Announcement
	if err := c.ShouldBindJSON(&announcement); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.AnnouncementServiceApp.Save(&announcement); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(announcement, c)
}

func (a AnnouncementApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.AnnouncementServiceApp.Delete(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
