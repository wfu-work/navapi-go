package apis

import (
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type GatewayApi struct{}

// Health 网关健康状态
// @Summary 网关健康状态
// @Description 网关健康状态
// @Tags 网关模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.GatewayHealth,msg=string}
// @Router /gateway/health [get]
func (a GatewayApi) Health(c *gin.Context) {
	response.Ok(services.GatewayServiceApp.Health(gin.Mode()), c)
}

// PublicStatus 公开服务状态
// @Summary 公开服务状态
// @Description 公开服务状态
// @Tags 网关模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.PublicServiceStatus,msg=string}
// @Router /service/status [get]
func (a GatewayApi) PublicStatus(c *gin.Context) {
	status, err := services.GatewayServiceApp.PublicStatus(gin.Mode())
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(status, c)
}
