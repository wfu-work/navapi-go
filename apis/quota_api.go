package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type QuotaApi struct{}

// Self 当前用户额度
// @Summary 当前用户额度
// @Description 当前用户额度
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=domains.UserQuota,msg=string}
// @Router /quota/self [get]
func (a QuotaApi) Self(c *gin.Context) {
	account, err := services.UserQuotaServiceApp.Get(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(account, c)
}

// List 用户额度列表
// @Summary 用户额度列表
// @Description 用户额度列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=dto.PageResult,msg=string}
// @Router /quota/list [get]
func (a QuotaApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.UserQuotaServiceApp.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Update 更新用户额度
// @Summary 更新用户额度
// @Description 更新用户额度
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.UserQuota true "用户额度对象"
// @Success 200 {object} response.Response{data=domains.UserQuota,msg=string}
// @Router /quota [put]
func (a QuotaApi) Update(c *gin.Context) {
	var account domains.UserQuota
	if err := c.ShouldBindJSON(&account); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.UserQuotaServiceApp.Update(&account); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(account, c)
}
