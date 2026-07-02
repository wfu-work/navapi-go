package apis

import (
	"strings"

	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type ProviderApi struct{}

type providerKeyRequest struct {
	Key string `json:"key" binding:"required"`
}

// List 上游提供商列表
// @Summary 上游提供商列表
// @Description 上游提供商列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Param type query string false "类型"
// @Param status query string false "状态 enabled/disabled"
// @Param keyStatus query string false "密钥状态 set/missing"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /provider/list [get]
func (a ProviderApi) List(c *gin.Context) {
	var query services.ProviderListQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.ProviderServiceApp.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Get 上游提供商详情
// @Summary 上游提供商详情
// @Description 上游提供商详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Success 200 {object} response.Response{data=domains.VendorMeta,msg=string}
// @Router /provider/{guid} [get]
func (a ProviderApi) Get(c *gin.Context) {
	guid := providerGuidParam(c)
	provider, err := services.ProviderServiceApp.GetByGUID(guid)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(services.ProviderRecordFromDomain(*provider), c)
}

// Save 上游提供商
// @Summary 上游提供商
// @Description 上游提供商
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.VendorMeta true "上游提供商对象"
// @Success 200 {object} response.Response{data=domains.VendorMeta,msg=string}
// @Router /provider [post]
// @Router /provider [put]
func (a ProviderApi) Save(c *gin.Context) {
	var provider domains.VendorMeta
	if err := c.ShouldBindJSON(&provider); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ProviderServiceApp.Save(&provider); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(services.ProviderRecordFromDomain(provider), c)
}

// Delete 删除上游提供商
// @Summary 删除上游提供商
// @Description 删除上游提供商
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /provider/{guid} [delete]
func (a ProviderApi) Delete(c *gin.Context) {
	if err := services.ProviderServiceApp.Delete(providerGuidParam(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Key 上游提供商密钥
// @Summary 上游提供商密钥
// @Description 上游提供商密钥
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /provider/{guid}/key [get]
func (a ProviderApi) Key(c *gin.Context) {
	key, err := services.ProviderServiceApp.GetKey(providerGuidParam(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(gin.H{"key": key}, c)
}

// SetKey 设置上游提供商密钥
// @Summary 设置上游提供商密钥
// @Description 设置上游提供商密钥
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Param data body providerKeyRequest true "上游提供商密钥对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /provider/{guid}/key [put]
func (a ProviderApi) SetKey(c *gin.Context) {
	var req providerKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ProviderServiceApp.SetKey(providerGuidParam(c), req.Key); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func providerGuidParam(c *gin.Context) string {
	return strings.TrimSpace(c.Param("guid"))
}
