package apis

import (
	"net/http"
	"strconv"

	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type AnnouncementApi struct{}

type announcementRequest struct {
	ID uint `json:"id"`
	domains.Announcement
}

type announcementResponse struct {
	ID uint `json:"id"`
	domains.Announcement
}

// PublicList 公开公告列表
// @Summary 公开公告列表
// @Description 公开公告列表
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /announcements/list [get]
func (a AnnouncementApi) PublicList(c *gin.Context) {
	var query services.AnnouncementQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.AnnouncementServiceApp.List(query, true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result.List = announcementResponses(result.List)
	response.Ok(result, c)
}

// PublicLatest 最新公告
// @Summary 最新公告
// @Description 最新公告
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Param limit query int false "返回数量"
// @Success 200 {object} response.Response{data=[]domains.Announcement,msg=string}
// @Router /announcements/latest [get]
func (a AnnouncementApi) PublicLatest(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	announcements, err := services.AnnouncementServiceApp.Latest(limit)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(announcementResponses(announcements), c)
}

// List 公告列表
// @Summary 公告列表
// @Description 公告列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Param status query int false "状态"
// @Param level query string false "级别"
// @Param popup query bool false "是否弹窗"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /announcement/list [get]
func (a AnnouncementApi) List(c *gin.Context) {
	var query services.AnnouncementQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.AnnouncementServiceApp.List(query, false)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result.List = announcementResponses(result.List)
	response.Ok(result, c)
}

// ClientList 客户端公告列表
// @Summary 客户端公告列表
// @Description 返回当前生效且启用的客户端公告
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Param level query string false "级别"
// @Param popup query bool false "是否弹窗"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /announcement/client/list [get]
func (a AnnouncementApi) ClientList(c *gin.Context) {
	var query services.AnnouncementQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.AnnouncementServiceApp.List(query, true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result.List = announcementResponses(result.List)
	response.Ok(result, c)
}

// Get 公告详情
// @Summary 公告详情
// @Description 公告详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.Announcement,msg=string}
// @Router /announcement/{id} [get]
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
	response.Ok(announcementResponseOf(*announcement), c)
}

// Save 保存公告
// @Summary 保存公告
// @Description 保存公告
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body announcementRequest true "公告对象"
// @Success 200 {object} response.Response{data=announcementResponse,msg=string}
// @Router /announcement [post]
// @Router /announcement [put]
// @Router /announcement/{id} [put]
func (a AnnouncementApi) Save(c *gin.Context) {
	var req announcementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	announcement := req.Announcement
	announcement.Id = req.ID
	if announcement.Id == 0 && c.Param("id") != "" {
		id, err := parseUintParam(c, "id")
		if err != nil {
			response.FailWithMessage(err.Error(), c)
			return
		}
		announcement.Id = id
	}
	if c.Request.Method == http.MethodPut && announcement.Id == 0 {
		response.FailWithMessage("id is required", c)
		return
	}
	if c.Request.Method == http.MethodPost {
		announcement.Id = 0
		announcement.Guid = ""
	}
	if err := services.AnnouncementServiceApp.Save(&announcement); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(announcementResponseOf(announcement), c)
}

// Delete 删除公告
// @Summary 删除公告
// @Description 删除公告
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /announcement/{id} [delete]
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

func announcementResponseOf(announcement domains.Announcement) announcementResponse {
	return announcementResponse{ID: announcement.Id, Announcement: announcement}
}

func announcementResponses(value any) any {
	switch announcements := value.(type) {
	case []domains.Announcement:
		result := make([]announcementResponse, 0, len(announcements))
		for _, announcement := range announcements {
			result = append(result, announcementResponseOf(announcement))
		}
		return result
	default:
		return value
	}
}
