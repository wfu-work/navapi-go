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

// PublicPlans 公开订阅套餐
// @Summary 公开订阅套餐
// @Description 公开订阅套餐
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /subscription/public/plans [get]
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

// Plans 订阅套餐列表
// @Summary 订阅套餐列表
// @Description 订阅套餐列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /subscription/plans [get]
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

// GetPlan 订阅套餐详情
// @Summary 订阅套餐详情
// @Description 订阅套餐详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.SubscriptionPlan,msg=string}
// @Router /subscription/plan/{id} [get]
func (a SubscriptionApi) GetPlan(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	plan, err := services.SubscriptionServiceApp.GetPlan(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(plan, c)
}

// SavePlan 保存订阅套餐
// @Summary 保存订阅套餐
// @Description 保存订阅套餐
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.SubscriptionPlan true "订阅套餐对象"
// @Success 200 {object} response.Response{data=domains.SubscriptionPlan,msg=string}
// @Router /subscription/plan [post]
// @Router /subscription/plan [put]
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

// DeletePlan 删除订阅套餐
// @Summary 删除订阅套餐
// @Description 删除订阅套餐
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /subscription/plan/{id} [delete]
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

// List 用户订阅列表
// @Summary 用户订阅列表
// @Description 用户订阅列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /subscription/list [get]
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

// Self 当前用户订阅列表
// @Summary 当前用户订阅列表
// @Description 当前用户订阅列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /subscription/self/list [get]
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

// Subscribe 订阅套餐
// @Summary 订阅套餐
// @Description 订阅套餐
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.SubscribeRequest true "订阅请求"
// @Success 200 {object} response.Response{data=domains.UserSubscription,msg=string}
// @Router /subscription/subscribe [post]
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
