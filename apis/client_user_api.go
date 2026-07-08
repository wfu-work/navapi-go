package apis

import (
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
