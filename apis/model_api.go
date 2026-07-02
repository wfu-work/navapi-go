package apis

import (
	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type ModelApi struct{}

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
	models, err := services.ModelServiceApp.ListMeta()
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
	if err := services.ModelServiceApp.UpsertMeta(&meta); err != nil {
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
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /models/{id} [delete]
func (a ModelApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ModelServiceApp.DeleteMeta(id); err != nil {
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
	vendors, err := services.ModelServiceApp.ListVendors(false)
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
	vendors, err := services.ModelServiceApp.ListVendors(true)
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
	if err := services.ModelServiceApp.UpsertVendor(&meta); err != nil {
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
	if err := services.ModelServiceApp.DeleteVendor(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
