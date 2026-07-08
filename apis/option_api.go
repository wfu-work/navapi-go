package apis

import (
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type OptionApi struct{}

type optionRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

// All 系统配置列表
// @Summary 系统配置列表
// @Description 系统配置列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /option/list [get]
func (a OptionApi) All(c *gin.Context) {
	options, err := optionService.All()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(options, c)
}

// Set 设置系统配置
// @Summary 设置系统配置
// @Description 设置系统配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body optionRequest true "系统配置对象"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /option [put]
func (a OptionApi) Set(c *gin.Context) {
	var req optionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := optionService.Set(req.Key, req.Value); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// RiskControl 风控配置
// @Summary 风控配置
// @Description 风控配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.RiskControlSettings,msg=string}
// @Router /option/risk_control [get]
func (a OptionApi) RiskControl(c *gin.Context) {
	response.Ok(riskControlService.Get(), c)
}

// SetRiskControl 设置风控配置
// @Summary 设置风控配置
// @Description 设置风控配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.RiskControlSettings true "风控配置对象"
// @Success 200 {object} response.Response{data=services.RiskControlSettings,msg=string}
// @Router /option/risk_control [put]
func (a OptionApi) SetRiskControl(c *gin.Context) {
	var settings services.RiskControlSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := riskControlService.Set(settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(riskControlService.Get(), c)
}

// RegisterSettings 注册配置
// @Summary 注册配置
// @Description 注册配置
// @Tags Navapi模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.RegisterSettings,msg=string}
// @Router /register/settings [get]
// @Router /option/register_settings [get]
func (a OptionApi) RegisterSettings(c *gin.Context) {
	response.Ok(registerSettingService.Get(), c)
}

// SetRegisterSettings 设置注册配置
// @Summary 设置注册配置
// @Description 设置注册配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.RegisterSettings true "注册配置对象"
// @Success 200 {object} response.Response{data=services.RegisterSettings,msg=string}
// @Router /option/register_settings [put]
func (a OptionApi) SetRegisterSettings(c *gin.Context) {
	var settings services.RegisterSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := registerSettingService.Set(settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(registerSettingService.Get(), c)
}

// Delete 删除系统配置
// @Summary 删除系统配置
// @Description 删除系统配置
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param key path string true "配置键"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /option/{key} [delete]
func (a OptionApi) Delete(c *gin.Context) {
	if err := optionService.Delete(c.Param("key")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
