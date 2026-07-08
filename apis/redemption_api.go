package apis

import (
	"navapi-go/domains"
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type RedemptionApi struct{}

type redeemRequest struct {
	Code string `json:"code" binding:"required"`
}

// List 兑换码列表
// @Summary 兑换码列表
// @Description 兑换码列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /redemption/list [get]
// @Router /card/list [get]
func (a RedemptionApi) List(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := redemptionService.List(query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Stats 兑换码统计
// @Summary 兑换码统计
// @Description 兑换码统计
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /redemption/stats [get]
// @Router /card/stats [get]
func (a RedemptionApi) Stats(c *gin.Context) {
	stats, err := redemptionService.Stats()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

// Get 兑换码详情
// @Summary 兑换码详情
// @Description 兑换码详情
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=domains.Redemption,msg=string}
// @Router /redemption/{id} [get]
// @Router /card/{id} [get]
func (a RedemptionApi) Get(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	redemption, err := redemptionService.Get(id)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}

// Create 创建兑换码
// @Summary 创建兑换码
// @Description 创建兑换码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Redemption true "兑换码对象"
// @Success 200 {object} response.Response{data=domains.Redemption,msg=string}
// @Router /redemption [post]
// @Router /card [post]
func (a RedemptionApi) Create(c *gin.Context) {
	var redemption domains.Redemption
	if err := c.ShouldBindJSON(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := redemptionService.Create(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}

// BatchCreate 批量创建兑换码
// @Summary 批量创建兑换码
// @Description 批量创建兑换码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body services.RedemptionBatchRequest true "批量创建兑换码请求"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /redemption/batch [post]
// @Router /card/batch [post]
func (a RedemptionApi) BatchCreate(c *gin.Context) {
	var req services.RedemptionBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	cards, err := redemptionService.BatchCreate(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(cards, c)
}

// Update 更新兑换码
// @Summary 更新兑换码
// @Description 更新兑换码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body domains.Redemption true "兑换码对象"
// @Success 200 {object} response.Response{data=domains.Redemption,msg=string}
// @Router /redemption [put]
// @Router /card [put]
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
	if err := redemptionService.Update(&redemption); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}

// Delete 删除兑换码
// @Summary 删除兑换码
// @Description 删除兑换码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} response.Response{data=bool,msg=string}
// @Router /redemption/{id} [delete]
// @Router /card/{id} [delete]
func (a RedemptionApi) Delete(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := redemptionService.Delete(id); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

// Redeem 兑换兑换码
// @Summary 兑换兑换码
// @Description 兑换兑换码
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param data body redeemRequest true "兑换请求"
// @Success 200 {object} response.Response{data=domains.Redemption,msg=string}
// @Router /redemption/redeem [post]
// @Router /card/redeem [post]
func (a RedemptionApi) Redeem(c *gin.Context) {
	var req redeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	redemption, err := redemptionService.Redeem(req.Code, utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(redemption, c)
}
