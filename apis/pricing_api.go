package apis

import (
	"navapi-go/domains"
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type PricingApi struct{}

// Public 公开计费列表
// @Summary 公开计费列表
// @Description 公开计费列表
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.Pricing,msg=string}
// @Router /pricing [get]
func (a PricingApi) Public(c *gin.Context) {
	pricing, err := services.PricingServiceApp.PublicList()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(pricing, c)
}

// List 计费列表
// @Summary 计费列表
// @Description 计费列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /pricing/list [get]
func (a PricingApi) List(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.PricingServiceApp.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Upsert 保存计费配置
// @Summary 保存计费配置
// @Description 保存计费配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Pricing true "计费配置对象"
// @Success 200 {object} response.Response{data=domains.Pricing,msg=string}
// @Router /pricing [post]
// @Router /pricing [put]
func (a PricingApi) Upsert(c *gin.Context) {
	var pricing domains.Pricing
	if err := c.ShouldBindJSON(&pricing); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.PricingServiceApp.Upsert(&pricing); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(pricing, c)
}

// Delete 删除计费配置
// @Summary 删除计费配置
// @Description 删除计费配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /pricing/{id} [delete]
func (a PricingApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.PricingServiceApp.Delete(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
