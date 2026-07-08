package apis

import (
	"navapi-go/services"

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
// @Param type query string false "流水类型"
// @Param source query string false "流水来源"
// @Param direction query string false "流水方向"
// @Param startTime query int false "开始时间"
// @Param endTime query int false "结束时间"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /wallet/self/records [get]
func (a WalletApi) SelfRecords(c *gin.Context) {
	var query services.WalletRecordQuery
	_ = c.ShouldBindQuery(&query)
	result, err := userWalletService.ListRecords(commonUtils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// SelfActivities 当前用户钱包活动
// @Summary 当前用户钱包活动
// @Description 当前用户钱包活动，API 消费按小时汇总，其它资金动作单条展示
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param startTime query int false "开始时间"
// @Param endTime query int false "结束时间"
// @Success 200 {object} response.Response{data=[]services.WalletActivityItem,msg=string}
// @Router /wallet/self/activities [get]
func (a WalletApi) SelfActivities(c *gin.Context) {
	var query services.WalletActivityQuery
	_ = c.ShouldBindQuery(&query)
	activities, err := userWalletService.ListActivities(commonUtils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(activities, c)
}
