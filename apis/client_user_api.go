package apis

import (
	"strings"

	"navapi-go/middlewares"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type ClientUserApi struct{}

// List 管理端用户列表
// @Summary 管理端用户列表
// @Description 支持按用户名、邮箱、手机号、昵称和 GUID 查询系统用户
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Param content query string false "兼容关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /clients/users/list [get]
func (a ClientUserApi) List(c *gin.Context) {
	var query services.ClientUserListQuery
	_ = c.ShouldBindQuery(&query)
	result, err := clientUserService.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Delete 删除管理端用户
// @Summary 删除管理端用户
// @Description 删除用户及其 API 密钥、配置、钱包、订单、订阅、任务、用量、签到、限额、兑换、邀请与消息数据
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Produce json
// @Param userGuid path string true "用户 GUID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /clients/users/{userGuid} [delete]
func (a ClientUserApi) Delete(c *gin.Context) {
	userGuid := strings.TrimSpace(c.Param("userGuid"))
	if userGuid == "" {
		response.FailWithMessage("user guid is required", c)
		return
	}
	if userGuid == middlewares.CurrentUserGuid(c) {
		response.FailWithMessage("the current user cannot be deleted", c)
		return
	}
	if err := clientUserService.Delete(userGuid); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
