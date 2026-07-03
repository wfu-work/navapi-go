package apis

import (
	"strconv"
	"strings"

	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type TokenApi struct{}

// List 令牌列表
// @Summary 令牌列表
// @Description 令牌列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.ApiToken,msg=string}
// @Router /token/list [get]
func (a TokenApi) List(c *gin.Context) {
	tokens, err := services.TokenServiceApp.List(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	for i := range tokens {
		tokens[i].MaskedKey = services.TokenServiceApp.Mask(tokens[i].Key)
	}
	response.Ok(tokens, c)
}

// Get 令牌详情
// @Summary 令牌详情
// @Description 令牌详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.ApiToken,msg=string}
// @Router /token/{id} [get]
func (a TokenApi) Get(c *gin.Context) {
	token, err := tokenByParam(c, utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.MaskedKey = services.TokenServiceApp.Mask(token.Key)
	response.Ok(token, c)
}

// Create 创建令牌
// @Summary 创建令牌
// @Description 创建令牌
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.ApiToken true "令牌对象"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /token [post]
func (a TokenApi) Create(c *gin.Context) {
	var token domains.ApiToken
	if err := c.ShouldBindJSON(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.UserGuid = utils.GetUserGuid(c)
	if err := services.TokenServiceApp.Create(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(gin.H{"token": token, "key": token.Key}, c)
}

// Update 更新令牌
// @Summary 更新令牌
// @Description 更新令牌
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.ApiToken true "令牌对象"
// @Success 200 {object} response.Response{data=domains.ApiToken,msg=string}
// @Router /token [put]
func (a TokenApi) Update(c *gin.Context) {
	var token domains.ApiToken
	if err := c.ShouldBindJSON(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if token.Id == 0 && strings.TrimSpace(token.Guid) == "" {
		response.FailWithMessage("guid is required", c)
		return
	}
	old, err := existingTokenForUpdate(token, utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.Key = old.Key
	token.UserGuid = old.UserGuid
	token.Guid = old.Guid
	if err := services.TokenServiceApp.Update(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.MaskedKey = services.TokenServiceApp.Mask(token.Key)
	response.Ok(token, c)
}

// Delete 删除令牌
// @Summary 删除令牌
// @Description 删除令牌
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /token/{id} [delete]
func (a TokenApi) Delete(c *gin.Context) {
	if err := deleteTokenByParam(c, utils.GetUserGuid(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Key 查看令牌密钥
// @Summary 查看令牌密钥
// @Description 查看令牌密钥
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /token/{id}/key [post]
func (a TokenApi) Key(c *gin.Context) {
	token, err := tokenByParam(c, utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(gin.H{"key": token.Key}, c)
}

func tokenByParam(c *gin.Context, userGuid string) (*domains.ApiToken, error) {
	raw := strings.TrimSpace(c.Param("id"))
	if raw == "" {
		return nil, strconv.ErrSyntax
	}
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id > 0 {
		return services.TokenServiceApp.GetByID(uint(id), userGuid)
	}
	return services.TokenServiceApp.GetByGUID(raw, userGuid)
}

func deleteTokenByParam(c *gin.Context, userGuid string) error {
	raw := strings.TrimSpace(c.Param("id"))
	if raw == "" {
		return strconv.ErrSyntax
	}
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id > 0 {
		return services.TokenServiceApp.Delete(uint(id), userGuid)
	}
	return services.TokenServiceApp.DeleteByGUID(raw, userGuid)
}

func existingTokenForUpdate(token domains.ApiToken, userGuid string) (*domains.ApiToken, error) {
	if strings.TrimSpace(token.Guid) != "" {
		return services.TokenServiceApp.GetByGUID(token.Guid, userGuid)
	}
	return services.TokenServiceApp.GetByID(token.Id, userGuid)
}

// Usage 当前用户令牌用量
// @Summary 当前用户令牌用量
// @Description 当前用户令牌用量
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /usage/token [get]
func (a TokenApi) Usage(c *gin.Context) {
	usage, err := services.TokenServiceApp.Usage(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(usage, c)
}
