package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type TaskApi struct{}

// Create 创建任务
// @Summary 创建任务
// @Description 创建任务
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Task true "任务对象"
// @Success 200 {object} response.Response{data=domains.Task,msg=string}
// @Router /task [post]
func (a TaskApi) Create(c *gin.Context) {
	var task domains.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.TaskServiceApp.Create(&task); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(task, c)
}

// CreateSelf 当前用户创建任务
// @Summary 当前用户创建任务
// @Description 当前用户创建任务
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Task true "任务对象"
// @Success 200 {object} response.Response{data=domains.Task,msg=string}
// @Router /task/self [post]
func (a TaskApi) CreateSelf(c *gin.Context) {
	var task domains.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	task.UserGuid = utils.GetUserGuid(c)
	if err := services.TaskServiceApp.Create(&task); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	task.PrivateData = ""
	response.Ok(task, c)
}

// Update 更新任务
// @Summary 更新任务
// @Description 更新任务
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Task true "任务对象"
// @Success 200 {object} response.Response{data=domains.Task,msg=string}
// @Router /task [put]
func (a TaskApi) Update(c *gin.Context) {
	var task domains.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.TaskServiceApp.Update(&task, ""); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(task, c)
}

// UpdateSelf 当前用户更新任务
// @Summary 当前用户更新任务
// @Description 当前用户更新任务
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Task true "任务对象"
// @Success 200 {object} response.Response{data=domains.Task,msg=string}
// @Router /task/self [put]
func (a TaskApi) UpdateSelf(c *gin.Context) {
	var task domains.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	task.UserGuid = utils.GetUserGuid(c)
	if err := services.TaskServiceApp.Update(&task, task.UserGuid); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	task.PrivateData = ""
	response.Ok(task, c)
}

// Delete 删除任务
// @Summary 删除任务
// @Description 删除任务
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /task/{task_id} [delete]
func (a TaskApi) Delete(c *gin.Context) {
	if err := services.TaskServiceApp.Delete(c.Param("task_id"), ""); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// DeleteSelf 当前用户删除任务
// @Summary 当前用户删除任务
// @Description 当前用户删除任务
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /task/self/{task_id} [delete]
func (a TaskApi) DeleteSelf(c *gin.Context) {
	if err := services.TaskServiceApp.Delete(c.Param("task_id"), utils.GetUserGuid(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// List 任务列表
// @Summary 任务列表
// @Description 任务列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /task/list [get]
func (a TaskApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.TaskServiceApp.List("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Self 当前用户任务列表
// @Summary 当前用户任务列表
// @Description 当前用户任务列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /task/self/list [get]
func (a TaskApi) Self(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.TaskServiceApp.List(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Get 任务详情
// @Summary 任务详情
// @Description 任务详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} response.Response{data=domains.Task,msg=string}
// @Router /task/{task_id} [get]
func (a TaskApi) Get(c *gin.Context) {
	task, err := services.TaskServiceApp.Get(c.Param("task_id"), "")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(task, c)
}

// GetSelf 当前用户任务详情
// @Summary 当前用户任务详情
// @Description 当前用户任务详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} response.Response{data=domains.Task,msg=string}
// @Router /task/self/{task_id} [get]
func (a TaskApi) GetSelf(c *gin.Context) {
	task, err := services.TaskServiceApp.Get(c.Param("task_id"), utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(task, c)
}
