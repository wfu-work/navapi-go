package apis

import (
	"strconv"

	"navapi-go/middlewares"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type UsageLogApi struct{}

// List 用量日志列表
// @Summary 用量日志列表
// @Description 用量日志列表
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Param source query string false "日志来源 user/probe"
// @Param includeProbe query bool false "是否包含探测日志"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /usage/list [get]
func (a UsageLogApi) List(c *gin.Context) {
	var query services.UsageLogQuery
	_ = c.ShouldBindQuery(&query)
	result, err := logService.List("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Self 当前用户用量日志
// @Summary 当前用户用量日志
// @Description 当前用户用量日志
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Param source query string false "日志来源 user/probe"
// @Param includeProbe query bool false "是否包含探测日志"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /usage/self/list [get]
func (a UsageLogApi) Self(c *gin.Context) {
	var query services.UsageLogQuery
	_ = c.ShouldBindQuery(&query)
	result, err := logService.List(middlewares.ScopedUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// Stats 用量统计
// @Summary 用量统计
// @Description 用量统计
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param source query string false "日志来源 user/probe"
// @Param includeProbe query bool false "是否包含探测日志"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /usage/stat [get]
func (a UsageLogApi) Stats(c *gin.Context) {
	var query services.UsageLogQuery
	_ = c.ShouldBindQuery(&query)
	stats, err := logService.Stats("", query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

// SelfStats 当前用户用量统计
// @Summary 当前用户用量统计
// @Description 当前用户用量统计
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param source query string false "日志来源 user/probe"
// @Param includeProbe query bool false "是否包含探测日志"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /usage/self/stat [get]
func (a UsageLogApi) SelfStats(c *gin.Context) {
	var query services.UsageLogQuery
	_ = c.ShouldBindQuery(&query)
	stats, err := logService.Stats(middlewares.ScopedUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(stats, c)
}

// Data 用量趋势数据
// @Summary 用量趋势数据
// @Description 用量趋势数据
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param days query int false "统计天数"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /data/list [get]
func (a UsageLogApi) Data(c *gin.Context) {
	data, err := logService.DailyData("", parseDaysQuery(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(data, c)
}

// SelfData 当前用户用量趋势数据
// @Summary 当前用户用量趋势数据
// @Description 当前用户用量趋势数据
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param days query int false "统计天数"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /data/self/list [get]
func (a UsageLogApi) SelfData(c *gin.Context) {
	data, err := logService.DailyData(middlewares.ScopedUserGuid(c), parseDaysQuery(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(data, c)
}

// UsageSummary 用量汇总
// @Summary 用量汇总
// @Description 用量汇总
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param days query int false "统计天数"
// @Param startTime query int false "开始时间"
// @Param endTime query int false "结束时间"
// @Param top query int false "返回数量"
// @Param source query string false "日志来源 user/probe"
// @Param includeProbe query bool false "是否包含探测日志"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /usage/summary [get]
func (a UsageLogApi) UsageSummary(c *gin.Context) {
	summary, err := logService.UsageSummaryByQuery("", parseUsageSummaryQuery(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(summary, c)
}

// SelfUsageSummary 当前用户用量汇总
// @Summary 当前用户用量汇总
// @Description 当前用户用量汇总
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param days query int false "统计天数"
// @Param startTime query int false "开始时间"
// @Param endTime query int false "结束时间"
// @Param top query int false "返回数量"
// @Param source query string false "日志来源 user/probe"
// @Param includeProbe query bool false "是否包含探测日志"
// @Success 200 {object} response.Response{data=object,msg=string}
// @Router /usage/self/summary [get]
func (a UsageLogApi) SelfUsageSummary(c *gin.Context) {
	summary, err := logService.UsageSummaryByQuery(middlewares.ScopedUserGuid(c), parseUsageSummaryQuery(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(summary, c)
}

func parseDaysQuery(c *gin.Context) int {
	days, err := strconv.Atoi(c.DefaultQuery("days", "7"))
	if err != nil {
		return 7
	}
	return days
}

func parseTopQuery(c *gin.Context) int {
	top, err := strconv.Atoi(c.DefaultQuery("top", "10"))
	if err != nil {
		return 10
	}
	return top
}

func parseUsageSummaryQuery(c *gin.Context) services.UsageSummaryQuery {
	return services.UsageSummaryQuery{
		Days:         parseDaysQuery(c),
		TopN:         parseTopQuery(c),
		StartTime:    parseInt64Query(c, "startTime"),
		EndTime:      parseInt64Query(c, "endTime"),
		Source:       c.Query("source"),
		IncludeProbe: parseBoolQuery(c, "includeProbe"),
	}
}

func parseInt64Query(c *gin.Context, key string) int64 {
	value, err := strconv.ParseInt(c.DefaultQuery(key, "0"), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseBoolQuery(c *gin.Context, key string) bool {
	value, err := strconv.ParseBool(c.DefaultQuery(key, "false"))
	if err != nil {
		return false
	}
	return value
}
