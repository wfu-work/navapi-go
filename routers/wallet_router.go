package routers

import "github.com/gin-gonic/gin"

type WalletRouter struct{}

func (r WalletRouter) InitWalletRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	group := privateGroup.Group("wallet")
	{
		group.GET("/self", walletApi.Self)
		group.GET("/self/records", walletApi.SelfRecords)
	}

	r.initWalletCompatRouter(publicGroup)
}

func (r WalletRouter) initWalletCompatRouter(group *gin.RouterGroup) {
	apiUser := group.Group("user")
	{
		apiUser.GET("/self", walletApi.NewAPIUserSelf)
		apiUser.GET("/balance", walletApi.BalanceCompat)
	}
	apiWallet := group.Group("wallet")
	{
		apiWallet.GET("/balance", walletApi.BalanceCompat)
	}
	rootUser := group.Group("../user")
	{
		rootUser.GET("/balance", walletApi.BalanceCompat)
	}
	rootWallet := group.Group("../wallet")
	{
		rootWallet.GET("/balance", walletApi.BalanceCompat)
	}
}
