package apis

import (
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
)

type WalletApi struct{}

// Self 当前用户钱包
// @Summary 当前用户钱包
// @Description 当前用户钱包
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=domains.UserWallet,msg=string}
// @Router /wallet/self [get]
func (a WalletApi) Self(c *gin.Context) {
	wallet, err := userWalletService.Get(commonUtils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(wallet, c)
}

// SelfRecords 当前用户钱包流水
// @Summary 当前用户钱包流水
// @Description 当前用户钱包流水
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /wallet/self/records [get]
func (a WalletApi) SelfRecords(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := userWalletService.ListRecords(commonUtils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}
