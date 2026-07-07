package routers

import "github.com/gin-gonic/gin"

type WalletRouter struct{}

func (r WalletRouter) InitWalletRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	group := privateGroup.Group("wallet")
	{
		group.GET("/self", walletApi.Self)
		group.GET("/self/records", walletApi.SelfRecords)
	}
}
