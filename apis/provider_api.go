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
