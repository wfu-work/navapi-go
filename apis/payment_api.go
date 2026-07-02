package apis

import (
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type PaymentApi struct{}

type closePaymentRequest struct {
	OrderNo string `json:"orderNo" binding:"required"`
}

// List 支付订单列表
// @Summary 支付订单列表
// @Description 支付订单列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /payment/list [get]
func (a PaymentApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.PaymentServiceApp.List("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Self 当前用户支付订单
// @Summary 当前用户支付订单
// @Description 当前用户支付订单
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /payment/self/list [get]
func (a PaymentApi) Self(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.PaymentServiceApp.List(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Create 创建支付订单
// @Summary 创建支付订单
// @Description 创建支付订单
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.CreatePaymentRequest true "创建支付订单请求"
// @Success 200 {object} response.Response{data=domains.PaymentOrder,msg=string}
// @Router /payment/create [post]
func (a PaymentApi) Create(c *gin.Context) {
	var req services.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	order, err := services.PaymentServiceApp.Create(utils.GetUserGuid(c), req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(order, c)
}

// Confirm 确认支付订单
// @Summary 确认支付订单
// @Description 确认支付订单
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.ConfirmPaymentRequest true "确认支付订单请求"
// @Success 200 {object} response.Response{data=domains.PaymentOrder,msg=string}
// @Router /payment/confirm [post]
func (a PaymentApi) Confirm(c *gin.Context) {
	var req services.ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	order, err := services.PaymentServiceApp.Confirm(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(order, c)
}

// Close 关闭当前用户支付订单
// @Summary 关闭当前用户支付订单
// @Description 关闭当前用户支付订单
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body closePaymentRequest true "关闭支付订单请求"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /payment/self/close [post]
func (a PaymentApi) Close(c *gin.Context) {
	var req closePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.PaymentServiceApp.Close(req.OrderNo, utils.GetUserGuid(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// AdminClose 关闭支付订单
// @Summary 关闭支付订单
// @Description 关闭支付订单
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body closePaymentRequest true "关闭支付订单请求"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /payment/close [post]
func (a PaymentApi) AdminClose(c *gin.Context) {
	var req closePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.PaymentServiceApp.Close(req.OrderNo, ""); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
