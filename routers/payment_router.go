package routers

import "github.com/gin-gonic/gin"

type PaymentRouter struct{}

func (r PaymentRouter) InitPaymentRouter(router *gin.RouterGroup) {
	group := router.Group("payment")
	{
		group.GET("/list", paymentApi.List)
		group.GET("/self/list", paymentApi.Self)
		group.POST("/create", paymentApi.Create)
		group.POST("/confirm", paymentApi.Confirm)
		group.POST("/close", paymentApi.AdminClose)
		group.POST("/self/close", paymentApi.Close)
	}
}
