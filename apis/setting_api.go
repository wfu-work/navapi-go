package apis

import (
	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"github.com/wfu-work/nav-common-go-lib/response"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
)

type SettingApi struct{}

// List 查询设置列表
// @Summary 查询设置列表
// @Description 分页查询运行时设置
// @Tags 设置模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param keyword query string false "搜索关键字"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /settings/list [get]
func (SettingApi) List(c *gin.Context) {
	params := queryParams(c)
	items, total, err := services.SettingServiceApp.List(params)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(commonDomains.PageResult{Data: items, Total: total, Page: commonUtils.Str2Int(params["page"]), Size: commonUtils.Str2Int(params["size"])}, c)
}

// Save 保存设置
// @Summary 保存设置
// @Description 创建或更新运行时设置
// @Tags 设置模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Setting true "设置信息"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /settings [post]
func (SettingApi) Save(c *gin.Context) {
	var setting domains.Setting
	if err := c.ShouldBindJSON(&setting); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.SettingServiceApp.Save(setting); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Contact 查询联系配置
// @Summary 查询联系配置
// @Description 查询 QQ、微信和赞助二维码配置
// @Tags 设置模块
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=services.ContactSettings,msg=string}
// @Router /contact/settings [get]
// @Router /settings/contact [get]
func (SettingApi) Contact(c *gin.Context) {
	settings, err := services.SettingServiceApp.ContactSettings()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(settings, c)
}

// SaveContact 保存联系配置
// @Summary 保存联系配置
// @Description 保存 QQ、微信和赞助二维码配置
// @Tags 设置模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.ContactSettings true "联系配置"
// @Success 200 {object} response.Response{data=services.ContactSettings,msg=string}
// @Router /settings/contact [put]
func (SettingApi) SaveContact(c *gin.Context) {
	var settings services.ContactSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	saved, err := services.SettingServiceApp.SaveContactSettings(settings)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(saved, c)
}

// Delete 删除设置
// @Summary 删除设置
// @Description 根据 GUID 删除运行时设置
// @Tags 设置模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param guid path string true "GUID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /settings/{guid} [delete]
func (SettingApi) Delete(c *gin.Context) {
	if err := services.SettingServiceApp.Delete(c.Param("guid")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
