package apis

import (
	"navapi-go/domains"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type ModelApi struct{}

type modelGroupStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

// List 模型列表
// @Summary 模型列表
// @Description 模型列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.ModelMeta,msg=string}
// @Router /models/list [get]
func (a ModelApi) List(c *gin.Context) {
	models, err := modelService.ListMeta()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(models, c)
}

// PublicList 公开模型元数据列表
// @Summary 公开模型元数据列表
// @Description 公开模型元数据列表
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.ModelMeta,msg=string}
// @Router /models/meta [get]
func (a ModelApi) PublicList(c *gin.Context) {
	models, err := modelService.PublicListMeta()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(models, c)
}

// Upsert 保存模型
// @Summary 保存模型
// @Description 保存模型
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.ModelMeta true "模型对象"
// @Success 200 {object} response.Response{data=domains.ModelMeta,msg=string}
// @Router /models [post]
// @Router /models [put]
func (a ModelApi) Upsert(c *gin.Context) {
	var meta domains.ModelMeta
	if err := c.ShouldBindJSON(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := modelService.UpsertMeta(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(meta, c)
}

// Delete 删除模型
// @Summary 删除模型
// @Description 删除模型
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /models/{guid} [delete]
func (a ModelApi) Delete(c *gin.Context) {
	if err := modelService.DeleteMeta(c.Param("guid")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Groups 模型分组列表
// @Summary 模型分组列表
// @Description 模型分组列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.ModelGroup,msg=string}
// @Router /models/groups [get]
func (a ModelApi) Groups(c *gin.Context) {
	groups, err := modelService.ListGroups(true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(groups, c)
}

// PublicGroups 公开模型分组列表
// @Summary 公开模型分组列表
// @Description 公开模型分组列表
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.ModelGroup,msg=string}
// @Router /models/group-options [get]
func (a ModelApi) PublicGroups(c *gin.Context) {
	groups, err := modelService.ListGroups(false)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(groups, c)
}

// UpsertGroup 保存模型分组
// @Summary 保存模型分组
// @Description 保存模型分组
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.ModelGroup true "模型分组对象"
// @Success 200 {object} response.Response{data=domains.ModelGroup,msg=string}
// @Router /models/groups [post]
// @Router /models/groups [put]
func (a ModelApi) UpsertGroup(c *gin.Context) {
	var group domains.ModelGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := modelService.UpsertGroup(&group); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(group, c)
}

// UpdateGroupStatus 更新模型分组启用状态
// @Summary 更新模型分组启用状态
// @Description 更新模型分组启用状态
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Param data body modelGroupStatusRequest true "模型分组状态"
// @Success 200 {object} response.Response{data=domains.ModelGroup,msg=string}
// @Router /models/groups/{guid}/status [patch]
func (a ModelApi) UpdateGroupStatus(c *gin.Context) {
	var request modelGroupStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if request.Enabled == nil {
		response.FailWithMessage("enabled is required", c)
		return
	}
	group, err := modelService.SetGroupEnabled(c.Param("guid"), *request.Enabled)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(group, c)
}

// DeleteGroup 删除模型分组
// @Summary 删除模型分组
// @Description 删除模型分组
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /models/groups/{guid} [delete]
func (a ModelApi) DeleteGroup(c *gin.Context) {
	if err := modelService.DeleteGroup(c.Param("guid")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// PublicVendors 公开供应商列表
// @Summary 公开供应商列表
// @Description 公开供应商列表
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.VendorMeta,msg=string}
// @Router /vendors [get]
func (a ModelApi) PublicVendors(c *gin.Context) {
	vendors, err := modelService.ListVendors(false)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(vendors, c)
}

// Vendors 供应商列表
// @Summary 供应商列表
// @Description 供应商列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.VendorMeta,msg=string}
// @Router /vendors/list [get]
func (a ModelApi) Vendors(c *gin.Context) {
	vendors, err := modelService.ListVendors(true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(vendors, c)
}

// UpsertVendor 保存供应商
// @Summary 保存供应商
// @Description 保存供应商
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.VendorMeta true "供应商对象"
// @Success 200 {object} response.Response{data=domains.VendorMeta,msg=string}
// @Router /vendors [post]
// @Router /vendors [put]
func (a ModelApi) UpsertVendor(c *gin.Context) {
	var meta domains.VendorMeta
	if err := c.ShouldBindJSON(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := modelService.UpsertVendor(&meta); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(meta, c)
}

// DeleteVendor 删除供应商
// @Summary 删除供应商
// @Description 删除供应商
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /vendors/{id} [delete]
func (a ModelApi) DeleteVendor(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := modelService.DeleteVendor(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
