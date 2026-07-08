package apis

import (
	"strconv"
	"strings"

	"navapi-go/domains"
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
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
	a.list(c, "")
}

// SelfList 当前用户令牌列表
// @Summary 当前用户令牌列表
// @Description 当前用户令牌列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]domains.ApiToken,msg=string}
// @Router /token/self/list [get]
func (a TokenApi) SelfList(c *gin.Context) {
	a.list(c, middlewares.ScopedUserGuid(c))
}

func (a TokenApi) list(c *gin.Context, userGuid string) {
	tokens, err := tokenService.List(userGuid)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	for i := range tokens {
		tokens[i].MaskedKey = tokenService.Mask(tokens[i].Key)
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
	a.get(c, "")
}

// SelfGet 当前用户令牌详情
// @Summary 当前用户令牌详情
// @Description 当前用户令牌详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.ApiToken,msg=string}
// @Router /token/self/{id} [get]
func (a TokenApi) SelfGet(c *gin.Context) {
	a.get(c, middlewares.ScopedUserGuid(c))
}

func (a TokenApi) get(c *gin.Context, userGuid string) {
	token, err := tokenByParam(c, userGuid)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.MaskedKey = tokenService.Mask(token.Key)
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
	a.create(c, "")
}

// CreateSelf 当前用户创建令牌
// @Summary 当前用户创建令牌
// @Description 当前用户创建令牌
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.ApiToken true "令牌对象"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /token/self [post]
func (a TokenApi) CreateSelf(c *gin.Context) {
	a.create(c, middlewares.ScopedUserGuid(c))
}

func (a TokenApi) create(c *gin.Context, userGuid string) {
	var token domains.ApiToken
	if err := c.ShouldBindJSON(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if userGuid != "" {
		token.UserGuid = userGuid
	} else if strings.TrimSpace(token.UserGuid) == "" {
		token.UserGuid = middlewares.CurrentUserGuid(c)
	}
	if err := tokenService.Create(&token); err != nil {
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
	a.update(c, "")
}

// UpdateSelf 当前用户更新令牌
// @Summary 当前用户更新令牌
// @Description 当前用户更新令牌
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.ApiToken true "令牌对象"
// @Success 200 {object} response.Response{data=domains.ApiToken,msg=string}
// @Router /token/self [put]
func (a TokenApi) UpdateSelf(c *gin.Context) {
	a.update(c, middlewares.ScopedUserGuid(c))
}

func (a TokenApi) update(c *gin.Context, userGuid string) {
	var token domains.ApiToken
	if err := c.ShouldBindJSON(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if token.Id == 0 && strings.TrimSpace(token.Guid) == "" {
		response.FailWithMessage("guid is required", c)
		return
	}
	old, err := existingTokenForUpdate(token, userGuid)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.Key = old.Key
	token.UserGuid = old.UserGuid
	token.Guid = old.Guid
	if err := tokenService.Update(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.MaskedKey = tokenService.Mask(token.Key)
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
	a.delete(c, "")
}

// DeleteSelf 当前用户删除令牌
// @Summary 当前用户删除令牌
// @Description 当前用户删除令牌
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /token/self/{id} [delete]
func (a TokenApi) DeleteSelf(c *gin.Context) {
	a.delete(c, middlewares.ScopedUserGuid(c))
}

func (a TokenApi) delete(c *gin.Context, userGuid string) {
	if err := deleteTokenByParam(c, userGuid); err != nil {
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
	a.key(c, "")
}

// KeySelf 当前用户查看令牌密钥
// @Summary 当前用户查看令牌密钥
// @Description 当前用户查看令牌密钥
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /token/self/{id}/key [post]
func (a TokenApi) KeySelf(c *gin.Context) {
	a.key(c, middlewares.ScopedUserGuid(c))
}

func (a TokenApi) key(c *gin.Context, userGuid string) {
	token, err := tokenByParam(c, userGuid)
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
		return tokenService.GetByID(uint(id), userGuid)
	}
	return tokenService.GetByGUID(raw, userGuid)
}

func deleteTokenByParam(c *gin.Context, userGuid string) error {
	raw := strings.TrimSpace(c.Param("id"))
	if raw == "" {
		return strconv.ErrSyntax
	}
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id > 0 {
		return tokenService.Delete(uint(id), userGuid)
	}
	return tokenService.DeleteByGUID(raw, userGuid)
}

func existingTokenForUpdate(token domains.ApiToken, userGuid string) (*domains.ApiToken, error) {
	if strings.TrimSpace(token.Guid) != "" {
		return tokenService.GetByGUID(token.Guid, userGuid)
	}
	return tokenService.GetByID(token.Id, userGuid)
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
	usage, err := tokenService.Usage(middlewares.ScopedUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(usage, c)
}
