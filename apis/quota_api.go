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

func (a QuotaApi) Self(c *gin.Context) {
	account, err := services.UserQuotaServiceApp.Get(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(account, c)
}

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
