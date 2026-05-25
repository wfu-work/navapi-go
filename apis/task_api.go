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

func (a TaskApi) Delete(c *gin.Context) {
	if err := services.TaskServiceApp.Delete(c.Param("task_id"), ""); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a TaskApi) DeleteSelf(c *gin.Context) {
	if err := services.TaskServiceApp.Delete(c.Param("task_id"), utils.GetUserGuid(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

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

func (a TaskApi) Get(c *gin.Context) {
	task, err := services.TaskServiceApp.Get(c.Param("task_id"), "")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(task, c)
}

func (a TaskApi) GetSelf(c *gin.Context) {
	task, err := services.TaskServiceApp.Get(c.Param("task_id"), utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(task, c)
}
