package routers

import "github.com/gin-gonic/gin"

type PaymentRouter struct{}

func (r PaymentRouter) InitPaymentRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicPayment := publicGroup.Group("payment")
	{
		publicPayment.POST("/wechat/notify", paymentApi.WechatNotify)
	}

	group := privateGroup.Group("payment")
	{
		group.GET("/list", paymentApi.List)
		group.GET("/self/list", paymentApi.Self)
		group.GET("/wechat/settings", paymentApi.WechatSettings)
		group.PUT("/wechat/settings", paymentApi.SetWechatSettings)
		group.POST("/create", paymentApi.Create)
		group.POST("/confirm", paymentApi.Confirm)
		group.POST("/close", paymentApi.AdminClose)
		group.POST("/self/close", paymentApi.Close)
	}
}
