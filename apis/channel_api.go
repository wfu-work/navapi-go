package apis

import (
	"strconv"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type ChannelApi struct{}

type channelFetchModelsRequest struct {
	ID     uint `json:"id" binding:"required"`
	Update bool `json:"update"`
}

type channelBatchRequest struct {
	IDs    []uint `json:"ids" binding:"required"`
	Status int    `json:"status" binding:"required"`
}

type channelTagRequest struct {
	Tag string `json:"tag" binding:"required"`
}

type channelKeyUpdateRequest struct {
	Key string `json:"key" binding:"required"`
}

type channelModelMappingRequest struct {
	Mapping map[string]string `json:"mapping" binding:"required"`
}

// List 渠道列表
// @Summary 渠道列表
// @Description 渠道列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.Channel,msg=string}
// @Router /channel/list [get]
func (a ChannelApi) List(c *gin.Context) {
	channels, err := services.ChannelServiceApp.List()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	for i := range channels {
		channels[i].Key = ""
	}
	response.Ok(channels, c)
}

// Get 渠道详情
// @Summary 渠道详情
// @Description 渠道详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.Channel,msg=string}
// @Router /channel/{id} [get]
func (a ChannelApi) Get(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	channel, err := services.ChannelServiceApp.GetByID(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	channel.Key = ""
	response.Ok(channel, c)
}

// Create 创建渠道
// @Summary 创建渠道
// @Description 创建渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Channel true "渠道对象"
// @Success 200 {object} response.Response{data=domains.Channel,msg=string}
// @Router /channel [post]
func (a ChannelApi) Create(c *gin.Context) {
	var channel domains.Channel
	if err := c.ShouldBindJSON(&channel); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.Create(&channel); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	channel.Key = ""
	response.Ok(channel, c)
}

// Update 更新渠道
// @Summary 更新渠道
// @Description 更新渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Channel true "渠道对象"
// @Success 200 {object} response.Response{data=domains.Channel,msg=string}
// @Router /channel [put]
func (a ChannelApi) Update(c *gin.Context) {
	var channel domains.Channel
	if err := c.ShouldBindJSON(&channel); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if channel.Id == 0 {
		response.FailWithMessage("id is required", c)
		return
	}
	old, err := services.ChannelServiceApp.GetByID(channel.Id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if channel.Key == "" {
		channel.Key = old.Key
	}
	if err := services.ChannelServiceApp.Update(&channel); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	channel.Key = ""
	response.Ok(channel, c)
}

// Delete 删除渠道
// @Summary 删除渠道
// @Description 删除渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/{id} [delete]
func (a ChannelApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.Delete(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Key 渠道密钥
// @Summary 渠道密钥
// @Description 渠道密钥
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /channel/{id}/key [get]
// @Router /channel/{id}/key [post]
func (a ChannelApi) Key(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	key, err := services.ChannelServiceApp.GetChannelKey(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(gin.H{"key": key}, c)
}

// SetKey 设置渠道密钥
// @Summary 设置渠道密钥
// @Description 设置渠道密钥
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Param data body channelKeyUpdateRequest true "渠道密钥对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/{id}/key [put]
func (a ChannelApi) SetKey(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var req channelKeyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.UpdateChannelKey(id, req.Key); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// UpdateUpstream 更新渠道上游配置
// @Summary 更新渠道上游配置
// @Description 更新渠道上游配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Param data body services.ChannelUpstreamConfig true "渠道上游配置"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/{id}/upstream [put]
func (a ChannelApi) UpdateUpstream(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var config services.ChannelUpstreamConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.UpdateUpstreamConfig(id, config); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// GetModelMapping 渠道模型映射
// @Summary 渠道模型映射
// @Description 渠道模型映射
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /channel/{id}/model_mapping [get]
func (a ChannelApi) GetModelMapping(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	mapping, err := services.ChannelServiceApp.GetModelMapping(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(gin.H{"mapping": mapping}, c)
}

// SetModelMapping 设置渠道模型映射
// @Summary 设置渠道模型映射
// @Description 设置渠道模型映射
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Param data body channelModelMappingRequest true "模型映射对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/{id}/model_mapping [put]
func (a ChannelApi) SetModelMapping(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var req channelModelMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.UpdateModelMapping(id, req.Mapping); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// HealthLogs 渠道健康日志
// @Summary 渠道健康日志
// @Description 渠道健康日志
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /channel/health_logs [get]
func (a ChannelApi) HealthLogs(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.ChannelServiceApp.ListHealthLogs(0, query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// ChannelHealthLogs 指定渠道健康日志
// @Summary 指定渠道健康日志
// @Description 指定渠道健康日志
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /channel/{id}/health_logs [get]
func (a ChannelApi) ChannelHealthLogs(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.ChannelServiceApp.ListHealthLogs(id, query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Test 测试渠道
// @Summary 测试渠道
// @Description 测试渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /channel/test/{id} [get]
func (a ChannelApi) Test(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result, err := services.ChannelServiceApp.Test(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Models 渠道可用模型
// @Summary 渠道可用模型
// @Description 渠道可用模型
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]string,msg=string}
// @Router /channel/models [get]
func (a ChannelApi) Models(c *gin.Context) {
	models, err := services.ChannelServiceApp.ListEnabledModels()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(models, c)
}

// FetchModels 拉取渠道模型
// @Summary 拉取渠道模型
// @Description 拉取渠道模型
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body channelFetchModelsRequest true "拉取模型请求"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /channel/fetch_models [post]
func (a ChannelApi) FetchModels(c *gin.Context) {
	var req channelFetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	models, err := services.ChannelServiceApp.FetchModels(req.ID, req.Update)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(models, c)
}

// Batch 批量更新渠道状态
// @Summary 批量更新渠道状态
// @Description 批量更新渠道状态
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body channelBatchRequest true "批量渠道状态对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/batch [post]
func (a ChannelApi) Batch(c *gin.Context) {
	var req channelBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.BatchStatus(req.IDs, req.Status); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// EnableByTag 按标签启用渠道
// @Summary 按标签启用渠道
// @Description 按标签启用渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body channelTagRequest true "渠道标签对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/tag/enabled [post]
func (a ChannelApi) EnableByTag(c *gin.Context) {
	a.setTagStatus(c, constants.StatusEnabled)
}

// DisableByTag 按标签禁用渠道
// @Summary 按标签禁用渠道
// @Description 按标签禁用渠道
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body channelTagRequest true "渠道标签对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /channel/tag/disabled [post]
func (a ChannelApi) DisableByTag(c *gin.Context) {
	a.setTagStatus(c, constants.StatusDisabled)
}

func (a ChannelApi) setTagStatus(c *gin.Context, status int) {
	var req channelTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.ChannelServiceApp.SetStatusByTag(req.Tag, status); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
