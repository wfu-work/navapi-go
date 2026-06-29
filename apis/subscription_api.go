package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type SubscriptionApi struct{}

func (a SubscriptionApi) PublicPlans(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.SubscriptionServiceApp.ListPlans(query, true)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a SubscriptionApi) Plans(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.SubscriptionServiceApp.ListPlans(query, false)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a SubscriptionApi) SavePlan(c *gin.Context) {
	var plan domains.SubscriptionPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.SubscriptionServiceApp.SavePlan(&plan); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(plan, c)
}

func (a SubscriptionApi) DeletePlan(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.SubscriptionServiceApp.DeletePlan(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a SubscriptionApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.SubscriptionServiceApp.ListUserSubscriptions("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a SubscriptionApi) Self(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.SubscriptionServiceApp.ListUserSubscriptions(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a SubscriptionApi) Subscribe(c *gin.Context) {
	var req services.SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	subscription, err := services.SubscriptionServiceApp.Subscribe(utils.GetUserGuid(c), req, "")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(subscription, c)
}
