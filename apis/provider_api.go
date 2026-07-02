package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
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
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /provider/list [get]
func (a ProviderApi) List(c *gin.Context) {
	var query dto.PageQuery
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
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.VendorMeta,msg=string}
// @Router /provider/{id} [get]
func (a ProviderApi) Get(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	provider, err := services.ProviderServiceApp.GetByID(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(provider, c)
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
	response.Ok(provider, c)
}

// Delete 删除上游提供商
// @Summary 删除上游提供商
// @Description 删除上游提供商
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /provider/{id} [delete]
func (a ProviderApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ProviderServiceApp.Delete(id); err != nil {
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
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /provider/{id}/key [get]
func (a ProviderApi) Key(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	key, err := services.ProviderServiceApp.GetKey(id)
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
// @Param id path int true "ID"
// @Param data body providerKeyRequest true "上游提供商密钥对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /provider/{id}/key [put]
func (a ProviderApi) SetKey(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var req providerKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ProviderServiceApp.SetKey(id, req.Key); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// CreateChannel 通过上游提供商创建渠道
// @Summary 通过上游提供商创建渠道
// @Description 通过上游提供商创建渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Param data body services.ProviderChannelRequest true "创建渠道请求"
// @Success 200 {object} response.Response{data=domains.Channel,msg=string}
// @Router /provider/{id}/channel [post]
func (a ProviderApi) CreateChannel(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var req services.ProviderChannelRequest
	_ = c.ShouldBindJSON(&req)
	channel, err := services.ProviderServiceApp.CreateChannel(id, req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(channel, c)
}
