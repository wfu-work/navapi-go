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
