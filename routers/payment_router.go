package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type PaymentRouter struct{}

func (r PaymentRouter) InitPaymentRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicPayment := publicGroup.Group("payment")
	{
		publicPayment.POST("/wechat/notify", paymentApi.WechatNotify)
	}

	group := privateGroup.Group("payment")
	{
		group.GET("/list", middlewares.AdminOnly(), paymentApi.List)
		group.GET("/self/list", paymentApi.Self)
		group.GET("/wechat/settings", middlewares.AdminOnly(), paymentApi.WechatSettings)
		group.PUT("/wechat/settings", middlewares.AdminOnly(), paymentApi.SetWechatSettings)
		group.POST("/create", paymentApi.Create)
		group.POST("/confirm", middlewares.AdminOnly(), paymentApi.Confirm)
		group.POST("/close", middlewares.AdminOnly(), paymentApi.AdminClose)
		group.POST("/self/close", paymentApi.Close)
	}
}
