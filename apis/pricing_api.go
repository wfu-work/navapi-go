package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type PricingApi struct{}

func (a PricingApi) Public(c *gin.Context) {
	pricing, err := services.PricingServiceApp.PublicList()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(pricing, c)
}

func (a PricingApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.PricingServiceApp.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

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
