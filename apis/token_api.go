package apis

import (
	"navapi-go/domains"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type TokenApi struct{}

func (a TokenApi) List(c *gin.Context) {
	tokens, err := services.TokenServiceApp.List(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	for i := range tokens {
		tokens[i].Key = services.TokenServiceApp.Mask(tokens[i].Key)
	}
	response.Ok(tokens, c)
}

func (a TokenApi) Get(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token, err := services.TokenServiceApp.GetByID(id, utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token.Key = services.TokenServiceApp.Mask(token.Key)
	response.Ok(token, c)
}

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

func (a TokenApi) Update(c *gin.Context) {
	var token domains.ApiToken
	if err := c.ShouldBindJSON(&token); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if token.Id == 0 {
		response.FailWithMessage("id is required", c)
		return
	}
	old, err := services.TokenServiceApp.GetByID(token.Id, utils.GetUserGuid(c))
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
	token.Key = services.TokenServiceApp.Mask(token.Key)
	response.Ok(token, c)
}

func (a TokenApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.TokenServiceApp.Delete(id, utils.GetUserGuid(c)); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a TokenApi) Key(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	token, err := services.TokenServiceApp.GetByID(id, utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(gin.H{"key": token.Key}, c)
}

func (a TokenApi) Usage(c *gin.Context) {
	usage, err := services.TokenServiceApp.Usage(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(usage, c)
}
