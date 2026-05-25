package apis

import (
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type RedemptionApi struct{}

type redeemRequest struct {
	Code    string `json:"code" binding:"required"`
	TokenID uint   `json:"tokenId" binding:"required"`
}

func (a RedemptionApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.RedemptionServiceApp.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a RedemptionApi) Create(c *gin.Context) {
	var redemption domains.Redemption
	if err := c.ShouldBindJSON(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.RedemptionServiceApp.Create(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}

func (a RedemptionApi) Update(c *gin.Context) {
	var redemption domains.Redemption
	if err := c.ShouldBindJSON(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if redemption.Id == 0 {
		response.FailWithMessage("id is required", c)
		return
	}
	if err := services.RedemptionServiceApp.Update(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}

func (a RedemptionApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.RedemptionServiceApp.Delete(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a RedemptionApi) Redeem(c *gin.Context) {
	var req redeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	redemption, err := services.RedemptionServiceApp.Redeem(req.Code, utils.GetUserGuid(c), req.TokenID)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}
