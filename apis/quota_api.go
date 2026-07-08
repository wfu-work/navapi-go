package apis

import (
	"navapi-go/domains"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type QuotaApi struct{}

// Self 当前用户余额账户
// @Summary 当前用户余额账户
// @Description 当前用户余额账户
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=domains.UserQuota,msg=string}
// @Router /balance/self [get]
func (a QuotaApi) Self(c *gin.Context) {
	account, err := userQuotaService.Get(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(account, c)
}

// List 用户余额账户列表
// @Summary 用户余额账户列表
// @Description 用户余额账户列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /balance/list [get]
func (a QuotaApi) List(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := userQuotaService.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Update 更新用户余额账户
// @Summary 更新用户余额账户
// @Description 更新用户余额账户
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.UserQuota true "用户余额账户对象"
// @Success 200 {object} response.Response{data=domains.UserQuota,msg=string}
// @Router /balance [put]
func (a QuotaApi) Update(c *gin.Context) {
	var account domains.UserQuota
	if err := c.ShouldBindJSON(&account); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := userQuotaService.Update(&account); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(account, c)
}
