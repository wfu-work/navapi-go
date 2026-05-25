package apis

import (
	"strconv"

	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	"github.com/wfu-work/nav-common-go-lib/utils"
)

type UsageLogApi struct{}

func (a UsageLogApi) List(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.LogServiceApp.List("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a UsageLogApi) Self(c *gin.Context) {
	var query dto.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.LogServiceApp.List(utils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a UsageLogApi) Stats(c *gin.Context) {
	stats, err := services.LogServiceApp.Stats("")
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

func (a UsageLogApi) SelfStats(c *gin.Context) {
	stats, err := services.LogServiceApp.Stats(utils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

func (a UsageLogApi) Data(c *gin.Context) {
	data, err := services.LogServiceApp.DailyData("", parseDaysQuery(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(data, c)
}

func (a UsageLogApi) SelfData(c *gin.Context) {
	data, err := services.LogServiceApp.DailyData(utils.GetUserGuid(c), parseDaysQuery(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(data, c)
}

func parseDaysQuery(c *gin.Context) int {
	days, err := strconv.Atoi(c.DefaultQuery("days", "7"))
	if err != nil {
		return 7
	}
	return days
}
