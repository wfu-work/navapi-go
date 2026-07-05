package routers

import "github.com/gin-gonic/gin"

type WalletRouter struct{}

func (r WalletRouter) InitWalletRouter(router *gin.RouterGroup) {
	group := router.Group("wallet")
	{
		group.GET("/self", walletApi.Self)
		group.GET("/self/records", walletApi.SelfRecords)
	}
}
