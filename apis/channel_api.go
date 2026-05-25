package apis

import (
	"strconv"

	"navapi-go/constants"
	"navapi-go/domains"
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

func (a ChannelApi) Models(c *gin.Context) {
	models, err := services.ChannelServiceApp.ListEnabledModels()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(models, c)
}

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

func (a ChannelApi) EnableByTag(c *gin.Context) {
	a.setTagStatus(c, constants.StatusEnabled)
}

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
