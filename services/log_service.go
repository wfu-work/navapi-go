package services

import (
	"encoding/json"
	"fmt"
	"time"

	"navapi-go/domains"
	"navapi-go/vos"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type LogService struct {
	commonServices.CrudService[domains.UsageLog]
}

var LogServiceApp = new(LogService)

func (s *LogService) WithDB(db *gorm.DB) *LogService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type DailyUsageData struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Quota    int64   `json:"quota"`
	Cost     float64 `json:"cost"`
	Tokens   int64   `json:"tokens"`
	Success  int64   `json:"success"`
	Errors   int64   `json:"errors"`
	UserGuid string  `json:"userGuid,omitempty"`
}

type UsageNamedSeries struct {
	Name      string           `json:"name"`
	ModelName string           `json:"modelName,omitempty"`
	Data      []DailyUsageData `json:"data"`
}

type UsageDimensionStat struct {
	Name                   string  `json:"name"`
	UserGuid               string  `json:"userGuid,omitempty"`
	Username               string  `json:"username,omitempty"`
	Email                  string  `json:"email,omitempty"`
	TokenGuid              string  `json:"tokenGuid,omitempty"`
	ProviderGuid           string  `json:"providerGuid,omitempty"`
	ModelName              string  `json:"modelName,omitempty"`
	Requests               int64   `json:"requests"`
	Success                int64   `json:"success"`
	Errors                 int64   `json:"errors"`
	Quota                  int64   `json:"quota"`
	Cost                   float64 `json:"cost"`
	PromptTokens           int64   `json:"promptTokens"`
	CompletionTokens       int64   `json:"completionTokens"`
	Tokens                 int64   `json:"tokens"`
	AvgUseTimeMs           int64   `json:"avgUseTimeMs"`
	AvgFirstResponseTimeMs int64   `json:"avgFirstResponseTimeMs"`
}

type UsageSummary struct {
	Days                   int                  `json:"days"`
	StartTime              int64                `json:"startTime,omitempty"`
	EndTime                int64                `json:"endTime,omitempty"`
	TotalRequests          int64                `json:"totalRequests"`
	SuccessRequests        int64                `json:"successRequests"`
	ErrorRequests          int64                `json:"errorRequests"`
	Quota                  int64                `json:"quota"`
	Cost                   float64              `json:"cost"`
	PromptTokens           int64                `json:"promptTokens"`
	CompletionTokens       int64                `json:"completionTokens"`
	Tokens                 int64                `json:"tokens"`
	AvgUseTimeMs           int64                `json:"avgUseTimeMs"`
	AvgFirstResponseTimeMs int64                `json:"avgFirstResponseTimeMs"`
	StreamRequests         int64                `json:"streamRequests"`
	Series                 []DailyUsageData     `json:"series"`
	SeriesByModel          []UsageNamedSeries   `json:"seriesByModel"`
	ByModel                []UsageDimensionStat `json:"byModel"`
	ByProvider             []UsageDimensionStat `json:"byProvider"`
	ByToken                []UsageDimensionStat `json:"byToken"`
	ByUser                 []UsageDimensionStat `json:"byUser,omitempty"`
}

type UsageSummaryQuery struct {
	Days      int
	TopN      int
	StartTime int64
	EndTime   int64
}

type UsageLogQuery struct {
	vos.PageQuery
	Status    string `form:"status" json:"status"`
	StartTime int64  `form:"startTime" json:"startTime"`
	EndTime   int64  `form:"endTime" json:"endTime"`
}

type usageAggregateRow struct {
	Requests            int64   `gorm:"column:requests"`
	Success             int64   `gorm:"column:success"`
	Errors              int64   `gorm:"column:errors"`
	Quota               int64   `gorm:"column:quota"`
	Cost                float64 `gorm:"column:cost"`
	PromptTokens        int64   `gorm:"column:prompt_tokens"`
	CompletionTokens    int64   `gorm:"column:completion_tokens"`
	UseTimeMs           int64   `gorm:"column:use_time_ms"`
	FirstResponseTimeMs int64   `gorm:"column:first_response_time_ms"`
	StreamRequests      int64   `gorm:"column:stream_requests"`
}

type usageDimensionAggregateRow struct {
	Key                 string  `gorm:"column:stat_key"`
	Name                string  `gorm:"column:name"`
	UserGuid            string  `gorm:"column:user_guid"`
	Username            string  `gorm:"column:username"`
	TokenGuid           string  `gorm:"column:token_guid"`
	ProviderGuid        string  `gorm:"column:provider_guid"`
	ModelName           string  `gorm:"column:model_name"`
	Requests            int64   `gorm:"column:requests"`
	Success             int64   `gorm:"column:success"`
	Errors              int64   `gorm:"column:errors"`
	Quota               int64   `gorm:"column:quota"`
	Cost                float64 `gorm:"column:cost"`
	PromptTokens        int64   `gorm:"column:prompt_tokens"`
	CompletionTokens    int64   `gorm:"column:completion_tokens"`
	UseTimeMs           int64   `gorm:"column:use_time_ms"`
	FirstResponseTimeMs int64   `gorm:"column:first_response_time_ms"`
}

type usageModelDailyAggregateRow struct {
	Date             string  `gorm:"column:usage_date"`
	ModelName        string  `gorm:"column:model_name"`
	Requests         int64   `gorm:"column:requests"`
	Success          int64   `gorm:"column:success"`
	Errors           int64   `gorm:"column:errors"`
	Quota            int64   `gorm:"column:quota"`
	Cost             float64 `gorm:"column:cost"`
	PromptTokens     int64   `gorm:"column:prompt_tokens"`
	CompletionTokens int64   `gorm:"column:completion_tokens"`
}

type usageDailyAggregateRow struct {
	Date                string  `gorm:"column:usage_date"`
	Requests            int64   `gorm:"column:requests"`
	Success             int64   `gorm:"column:success"`
	Errors              int64   `gorm:"column:errors"`
	Quota               int64   `gorm:"column:quota"`
	Cost                float64 `gorm:"column:cost"`
	PromptTokens        int64   `gorm:"column:prompt_tokens"`
	CompletionTokens    int64   `gorm:"column:completion_tokens"`
	UseTimeMs           int64   `gorm:"column:use_time_ms"`
	FirstResponseTimeMs int64   `gorm:"column:first_response_time_ms"`
	StreamRequests      int64   `gorm:"column:stream_requests"`
}

type usageDailyStatsResult struct {
	ByDate    map[string]DailyUsageData
	Aggregate usageAggregateRow
}

type usageDayWindow struct {
	Date string
}

func (s *LogService) Create(log *domains.UsageLog) error {
	return createWithCrud(&s.CrudService, log)
}

func (s *LogService) EnsureIndexes() error {
	db := s.DB()
	indexes := []struct {
		name string
		sql  string
	}{
		{name: "idx_nav_api_usage_logs_create_time", sql: "CREATE INDEX idx_nav_api_usage_logs_create_time ON nav_api_usage_logs(create_time)"},
		{name: "idx_nav_api_usage_logs_user_time", sql: "CREATE INDEX idx_nav_api_usage_logs_user_time ON nav_api_usage_logs(user_guid, create_time)"},
		{name: "idx_nav_api_usage_logs_model_time", sql: "CREATE INDEX idx_nav_api_usage_logs_model_time ON nav_api_usage_logs(model_name, create_time)"},
		{name: "idx_nav_api_usage_logs_status_time", sql: "CREATE INDEX idx_nav_api_usage_logs_status_time ON nav_api_usage_logs(status, create_time)"},
		{name: "idx_nav_api_usage_logs_user_status_time", sql: "CREATE INDEX idx_nav_api_usage_logs_user_status_time ON nav_api_usage_logs(user_guid, status, create_time)"},
		{name: "idx_nav_api_usage_logs_time_token", sql: "CREATE INDEX idx_nav_api_usage_logs_time_token ON nav_api_usage_logs(create_time, token_guid)"},
		{name: "idx_nav_api_usage_logs_time_channel", sql: "CREATE INDEX idx_nav_api_usage_logs_time_channel ON nav_api_usage_logs(create_time, channel_guid)"},
		{name: "idx_nav_api_usage_logs_source_time", sql: "CREATE INDEX idx_nav_api_usage_logs_source_time ON nav_api_usage_logs(source, create_time)"},
	}
	for _, index := range indexes {
		if db.Migrator().HasIndex(&domains.UsageLog{}, index.name) {
			continue
		}
		if err := db.Exec(index.sql).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *LogService) List(userGuid string, query UsageLogQuery) (vos.PageResult, error) {
	query.PageQuery.Normalize()
	var logs []domains.UsageLog
	var total int64
	db := s.DB().Model(&domains.UsageLog{})
	db = applyUsageLogFilters(db, userGuid, query)
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.PageQuery.Offset()).Limit(query.Size).Find(&logs).Error; err != nil {
		return vos.PageResult{}, err
	}
	s.enrichUsageLogUsers(logs)
	s.enrichOfficialCosts(logs)
	return vos.PageResult{List: logs, Total: total, Page: query.Page, Size: query.Size}, nil
}

func applyUsageLogFilters(db *gorm.DB, userGuid string, query UsageLogQuery) *gorm.DB {
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		userSubQuery := db.Session(&gorm.Session{NewDB: true}).
			Model(&commonDomains.SysUser{}).
			Select("guid").
			Where("username LIKE ? OR email LIKE ? OR nick_name LIKE ? OR guid LIKE ?", keyword, keyword, keyword, keyword)
		db = db.Where("model_name LIKE ? OR token_name LIKE ? OR channel_name LIKE ? OR user_guid LIKE ? OR username LIKE ? OR request_id LIKE ? OR upstream_request_id LIKE ? OR user_guid IN (?)", keyword, keyword, keyword, keyword, keyword, keyword, keyword, userSubQuery)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	db = applyUserUsageLogSourceFilter(db)
	startTime := normalizeUsageQueryTime(query.StartTime)
	endTime := normalizeUsageQueryTime(query.EndTime)
	if startTime > 0 {
		db = db.Where("create_time >= ?", startTime)
	}
	if endTime > 0 {
		db = db.Where("create_time <= ?", endTime)
	}
	return db
}

func applyUserUsageLogSourceFilter(db *gorm.DB) *gorm.DB {
	return db.Where("source IS NULL OR source = '' OR source = ?", domains.UsageLogSourceUser)
}

func (s *LogService) enrichUsageLogUsers(logs []domains.UsageLog) {
	userGuids := make([]string, 0, len(logs))
	seen := map[string]bool{}
	for _, log := range logs {
		if log.UserGuid == "" || seen[log.UserGuid] {
			continue
		}
		seen[log.UserGuid] = true
		userGuids = append(userGuids, log.UserGuid)
	}
	if len(userGuids) == 0 {
		return
	}
	var users []commonDomains.SysUser
	if err := s.DB().Where("guid IN ?", userGuids).Find(&users).Error; err != nil {
		return
	}
	userByGuid := make(map[string]commonDomains.SysUser, len(users))
	for _, user := range users {
		userByGuid[user.Guid] = user
	}
	for i := range logs {
		user, ok := userByGuid[logs[i].UserGuid]
		if !ok {
			continue
		}
		if user.Username != "" {
			logs[i].Username = user.Username
		}
		if user.Email != "" {
			logs[i].Email = user.Email
		}
	}
}

func normalizeUsageQueryTime(value int64) int64 {
	if value > 0 && value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

func (s *LogService) enrichOfficialCosts(logs []domains.UsageLog) {
	extras := make([]map[string]any, len(logs))
	modelNames := map[string]struct{}{}
	groupNames := map[string]struct{}{}
	for i := range logs {
		extra := usageLogExtraMap(logs[i].Other)
		extras[i] = extra
		if _, ok := extra["finalCost"]; ok {
			if logs[i].Cost <= 0 {
				logs[i].Cost = extraNumber(extra["finalCost"])
			}
			continue
		}
		group := normalizeGroup(extraText(extra["group"]))
		if logs[i].ModelName != "" && (logs[i].PromptTokens > 0 || logs[i].CompletionTokens > 0) {
			modelNames[logs[i].ModelName] = struct{}{}
			groupNames[group] = struct{}{}
		}
	}
	lookup := s.usageCostLookup(modelNames, groupNames)
	for i := range logs {
		extra := extras[i]
		if _, ok := extra["finalCost"]; ok {
			continue
		}
		group := normalizeGroup(extraText(extra["group"]))
		usage := vos.Usage{
			PromptTokens:     logs[i].PromptTokens,
			CompletionTokens: logs[i].CompletionTokens,
			CachedTokens:     int64(extraNumber(extra["cachedTokens"])),
		}
		detail := lookup.officialCostDetail(logs[i].ModelName, group, usage)
		if !detail.OfficialPricing {
			continue
		}
		extra["billingMode"] = detail.BillingMode
		extra["pricingMatched"] = detail.PricingMatched
		extra["pricingModel"] = detail.PricingModel
		extra["pricingGroup"] = detail.PricingGroup
		extra["groupMultiplier"] = detail.GroupMultiplier
		extra["regularPromptTokens"] = detail.RegularPromptTokens
		extra["cachedTokens"] = detail.CachedTokens
		extra["completionTokens"] = detail.CompletionTokens
		extra["officialPricing"] = detail.OfficialPricing
		extra["officialProvider"] = detail.OfficialProvider
		extra["officialPriceUnit"] = detail.OfficialPriceUnit
		extra["officialInputPrice"] = detail.OfficialInputPrice
		extra["officialOutputPrice"] = detail.OfficialOutputPrice
		extra["officialCachePrice"] = detail.OfficialCachePrice
		extra["priceUnitTokens"] = detail.PriceUnitTokens
		extra["rawCost"] = detail.RawCost
		extra["finalCost"] = detail.FinalCost
		logs[i].Cost = detail.FinalCost
		data, err := json.Marshal(extra)
		if err == nil {
			logs[i].Other = string(data)
		}
	}
}

type usageCostLookup struct {
	groupMultipliers map[string]float64
	officialMetas    map[string]domains.ModelMeta
}

func (s *LogService) usageCostLookup(modelNames map[string]struct{}, groupNames map[string]struct{}) usageCostLookup {
	lookup := usageCostLookup{
		groupMultipliers: map[string]float64{},
		officialMetas:    map[string]domains.ModelMeta{},
	}
	db := s.DB()
	if db == nil {
		return lookup
	}
	if len(groupNames) > 0 {
		groups := make([]string, 0, len(groupNames))
		for group := range groupNames {
			groups = append(groups, group)
		}
		var rows []domains.ModelGroup
		if err := db.Where("enabled = ? AND group_name IN ?", true, groups).Find(&rows).Error; err == nil {
			for _, row := range rows {
				multiplier := row.QuotaMultiplier
				if multiplier <= 0 {
					multiplier = 1
				}
				lookup.groupMultipliers[normalizeGroup(row.GroupName)] = multiplier
			}
		}
	}
	if len(modelNames) > 0 {
		models := make([]string, 0, len(modelNames))
		for model := range modelNames {
			models = append(models, model)
		}
		var rows []domains.ModelMeta
		if err := db.Where("enabled = ? AND model_name IN ?", true, models).Find(&rows).Error; err == nil {
			for _, row := range rows {
				if row.OfficialInputPrice <= 0 && row.OfficialOutputPrice <= 0 && row.OfficialCachePrice <= 0 {
					continue
				}
				if row.OfficialPriceUnit == "" {
					row.OfficialPriceUnit = "1M tokens"
				}
				lookup.officialMetas[row.ModelName] = row
			}
		}
	}
	return lookup
}

func (lookup usageCostLookup) groupMultiplier(group string) float64 {
	multiplier := lookup.groupMultipliers[normalizeGroup(group)]
	if multiplier <= 0 {
		return 1
	}
	return multiplier
}

func (lookup usageCostLookup) officialCostDetail(modelName string, group string, usage vos.Usage) QuotaCalculationDetail {
	groupMultiplier := lookup.groupMultiplier(group)
	detail := QuotaCalculationDetail{
		BillingMode:      "official_price",
		PromptMultiplier: 1,
		OutputMultiplier: 1,
		CacheMultiplier:  1,
		QuotaMultiplier:  1,
		GroupMultiplier:  groupMultiplier,
		CachedTokens:     usage.CachedTokens,
		CompletionTokens: usage.CompletionTokens,
	}
	meta, ok := lookup.officialMetas[modelName]
	if !ok || (usage.PromptTokens <= 0 && usage.CompletionTokens <= 0) {
		return detail
	}
	unitTokens := officialPriceUnitTokens(meta.OfficialPriceUnit)
	cachedTokens := usage.CachedTokens
	if cachedTokens > usage.PromptTokens {
		cachedTokens = usage.PromptTokens
	}
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	regularPromptTokens := usage.PromptTokens - cachedTokens
	if regularPromptTokens < 0 {
		regularPromptTokens = 0
	}
	rawCost := float64(regularPromptTokens)*meta.OfficialInputPrice/unitTokens +
		float64(cachedTokens)*meta.OfficialCachePrice/unitTokens +
		float64(usage.CompletionTokens)*meta.OfficialOutputPrice/unitTokens
	if rawCost <= 0 {
		return detail
	}
	detail.PricingMatched = true
	detail.PricingModel = meta.ModelName
	detail.PricingGroup = normalizeGroup(group)
	detail.OfficialPricing = true
	detail.OfficialProvider = meta.OfficialProvider
	detail.OfficialPriceUnit = meta.OfficialPriceUnit
	detail.OfficialInputPrice = meta.OfficialInputPrice
	detail.OfficialOutputPrice = meta.OfficialOutputPrice
	detail.OfficialCachePrice = meta.OfficialCachePrice
	detail.PriceUnitTokens = unitTokens
	detail.RegularPromptTokens = regularPromptTokens
	detail.CachedTokens = cachedTokens
	detail.CompletionTokens = usage.CompletionTokens
	detail.RawCost = rawCost
	detail.FinalCost = rawCost * detail.GroupMultiplier
	return detail
}

func usageLogExtraMap(raw string) map[string]any {
	extra := map[string]any{}
	if raw == "" {
		return extra
	}
	_ = json.Unmarshal([]byte(raw), &extra)
	return extra
}

func extraText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func extraNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		number, _ := typed.Float64()
		return number
	default:
		return 0
	}
}

func (s *LogService) Stats(userGuid string, filters ...UsageLogQuery) (map[string]any, error) {
	db := s.DB().Model(&domains.UsageLog{})
	if len(filters) > 0 {
		db = applyUsageLogFilters(db, userGuid, filters[0])
	} else {
		db = applyUsageLogFilters(db, userGuid, UsageLogQuery{})
	}
	aggregate, err := s.aggregateUsage(db)
	if err != nil {
		return nil, err
	}
	avgUseTimeMs := int64(0)
	if aggregate.Requests > 0 {
		avgUseTimeMs = aggregate.UseTimeMs / aggregate.Requests
	}
	return map[string]any{
		"totalRequests":          aggregate.Requests,
		"successRequests":        aggregate.Success,
		"errorRequests":          aggregate.Errors,
		"quota":                  aggregate.Quota,
		"cost":                   aggregate.Cost,
		"promptTokens":           aggregate.PromptTokens,
		"completionTokens":       aggregate.CompletionTokens,
		"tokens":                 aggregate.PromptTokens + aggregate.CompletionTokens,
		"avgUseTimeMs":           avgUseTimeMs,
		"avgFirstResponseTimeMs": avgFirstResponseTime(aggregate.FirstResponseTimeMs, aggregate.Requests),
	}, nil
}

func (s *LogService) DailyData(userGuid string, days int) ([]DailyUsageData, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	end := time.Now()
	start := beginningOfDay(end).AddDate(0, 0, -(days - 1))
	return s.dailyDataInRange(userGuid, start.UnixMilli(), end.UnixMilli())
}

func (s *LogService) dailyDataInRange(userGuid string, startTime int64, endTime int64) ([]DailyUsageData, error) {
	startTime = normalizeUsageQueryTime(startTime)
	endTime = normalizeUsageQueryTime(endTime)
	if endTime <= 0 {
		endTime = time.Now().UnixMilli()
	}
	if startTime <= 0 {
		startTime = beginningOfDay(time.UnixMilli(endTime)).AddDate(0, 0, -6).UnixMilli()
	}
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}
	windows := buildUsageDayWindows(startTime, endTime)
	stats, err := s.dailyUsageStats(userGuid, UsageSummaryQuery{StartTime: startTime, EndTime: endTime})
	if err != nil {
		return nil, err
	}
	return buildDailyUsageSeries(userGuid, windows, stats.ByDate), nil
}

// UsageSummary builds dashboard-ready aggregates from database-side summaries.
func (s *LogService) UsageSummary(userGuid string, days int, topN int) (UsageSummary, error) {
	return s.UsageSummaryByQuery(userGuid, UsageSummaryQuery{Days: days, TopN: topN})
}

func (s *LogService) UsageSummaryByQuery(userGuid string, query UsageSummaryQuery) (UsageSummary, error) {
	normalized := normalizeUsageSummaryQuery(query)
	now := time.Now()
	cacheKey := usageSummaryCacheKey(userGuid, query, normalized)
	if cached, ok := usageSummaryCacheGet(cacheKey, now); ok {
		return cached, nil
	}
	dailyStats, err := s.dailyUsageStats(userGuid, normalized)
	if err != nil {
		return UsageSummary{}, err
	}
	series := buildDailyUsageSeries(userGuid, buildUsageDayWindows(normalized.StartTime, normalized.EndTime), dailyStats.ByDate)
	summary := UsageSummary{Days: normalized.Days, StartTime: normalized.StartTime, EndTime: normalized.EndTime, Series: series}
	applyAggregateToSummary(&summary, dailyStats.Aggregate)
	summary.ByModel, err = s.usageDimensionStats(userGuid, normalized, "COALESCE(NULLIF(model_name, ''), 'unknown')", "COALESCE(NULLIF(model_name, ''), 'unknown')", "MAX(model_name) as model_name")
	if err != nil {
		return UsageSummary{}, err
	}
	summary.SeriesByModel, err = s.usageModelSeries(userGuid, normalized, summary.ByModel, series)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.ByProvider, err = s.usageDimensionStats(userGuid, normalized, "COALESCE(NULLIF(channel_guid, ''), NULLIF(channel_name, ''), 'unknown')", "COALESCE(NULLIF(channel_name, ''), NULLIF(channel_guid, ''), 'unknown')", "MAX(channel_guid) as provider_guid")
	if err != nil {
		return UsageSummary{}, err
	}
	summary.ByToken, err = s.usageDimensionStats(userGuid, normalized, "COALESCE(NULLIF(token_guid, ''), NULLIF(token_name, ''), 'unknown')", "COALESCE(NULLIF(token_name, ''), NULLIF(token_guid, ''), 'unknown')", "MAX(token_guid) as token_guid, MAX(user_guid) as user_guid")
	if err != nil {
		return UsageSummary{}, err
	}
	if userGuid == "" {
		summary.ByUser, err = s.usageDimensionStats(userGuid, normalized, "COALESCE(NULLIF(user_guid, ''), NULLIF(username, ''), 'unknown')", "COALESCE(NULLIF(username, ''), NULLIF(user_guid, ''), 'unknown')", "MAX(user_guid) as user_guid, MAX(username) as username")
		if err != nil {
			return UsageSummary{}, err
		}
		s.enrichUsageUsers(summary.ByUser)
	}
	usageSummaryCacheSet(cacheKey, summary, now)
	return summary, nil
}

func (s *LogService) usageRangeDB(userGuid string, query UsageSummaryQuery) *gorm.DB {
	startTime := normalizeUsageQueryTime(query.StartTime)
	endTime := normalizeUsageQueryTime(query.EndTime)
	db := s.DB().Model(&domains.UsageLog{}).Where("create_time >= ? AND create_time <= ?", startTime, endTime)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	return applyUserUsageLogSourceFilter(db)
}

func (s *LogService) dailyUsageStats(userGuid string, query UsageSummaryQuery) (usageDailyStatsResult, error) {
	db := s.usageRangeDB(userGuid, query)
	dateExpr := usageDateExprSQL(db)
	selectSQL := fmt.Sprintf(`
		%s as usage_date,
		COUNT(*) as requests,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 0 ELSE 1 END), 0) as errors,
		COALESCE(SUM(quota), 0) as quota,
		%s as cost,
		COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as completion_tokens,
		COALESCE(SUM(use_time_ms), 0) as use_time_ms,
		%s as first_response_time_ms,
		COALESCE(SUM(CASE WHEN is_stream THEN 1 ELSE 0 END), 0) as stream_requests
		`, dateExpr, usageCostSumSQL(db), usageFirstResponseTimeSumSQL())
	var rows []usageDailyAggregateRow
	if err := db.Select(selectSQL).Group(dateExpr).Scan(&rows).Error; err != nil {
		return usageDailyStatsResult{}, err
	}
	result := usageDailyStatsResult{ByDate: make(map[string]DailyUsageData, len(rows))}
	for _, row := range rows {
		if row.Date == "" {
			continue
		}
		result.ByDate[row.Date] = DailyUsageData{
			Date:     row.Date,
			UserGuid: userGuid,
			Requests: row.Requests,
			Quota:    row.Quota,
			Cost:     row.Cost,
			Tokens:   row.PromptTokens + row.CompletionTokens,
			Success:  row.Success,
			Errors:   row.Errors,
		}
		result.Aggregate.Requests += row.Requests
		result.Aggregate.Success += row.Success
		result.Aggregate.Errors += row.Errors
		result.Aggregate.Quota += row.Quota
		result.Aggregate.Cost += row.Cost
		result.Aggregate.PromptTokens += row.PromptTokens
		result.Aggregate.CompletionTokens += row.CompletionTokens
		result.Aggregate.UseTimeMs += row.UseTimeMs
		result.Aggregate.FirstResponseTimeMs += row.FirstResponseTimeMs
		result.Aggregate.StreamRequests += row.StreamRequests
	}
	return result, nil
}

func (s *LogService) aggregateUsage(db *gorm.DB) (usageAggregateRow, error) {
	var row usageAggregateRow
	selectSQL := fmt.Sprintf(`
		COUNT(*) as requests,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 0 ELSE 1 END), 0) as errors,
		COALESCE(SUM(quota), 0) as quota,
		%s as cost,
		COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as completion_tokens,
		COALESCE(SUM(use_time_ms), 0) as use_time_ms,
		%s as first_response_time_ms,
		COALESCE(SUM(CASE WHEN is_stream THEN 1 ELSE 0 END), 0) as stream_requests
		`, usageCostSumSQL(db), usageFirstResponseTimeSumSQL())
	return row, db.Select(selectSQL).Scan(&row).Error
}

func (s *LogService) usageDimensionStats(userGuid string, query UsageSummaryQuery, keyExpr string, nameExpr string, extraSelect string) ([]UsageDimensionStat, error) {
	db := s.usageRangeDB(userGuid, query)
	extra := ""
	if extraSelect != "" {
		extra = ", " + extraSelect
	}
	selectSQL := fmt.Sprintf(`
		%s as stat_key,
		%s as name%s,
		COUNT(*) as requests,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 0 ELSE 1 END), 0) as errors,
		COALESCE(SUM(quota), 0) as quota,
		%s as cost,
		COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as completion_tokens,
		COALESCE(SUM(use_time_ms), 0) as use_time_ms,
		%s as first_response_time_ms
		`, keyExpr, nameExpr, extra, usageCostSumSQL(db), usageFirstResponseTimeSumSQL())
	var rows []usageDimensionAggregateRow
	if err := db.Select(selectSQL).
		Group(keyExpr + ", " + nameExpr).
		Order(usageCostSumSQL(db) + " DESC").
		Order("COALESCE(SUM(quota), 0) DESC").
		Order("COUNT(*) DESC").
		Limit(query.TopN).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]UsageDimensionStat, 0, len(rows))
	for _, row := range rows {
		stat := row.toUsageDimensionStat()
		if stat.Requests > 0 {
			stat.AvgUseTimeMs = stat.AvgUseTimeMs / stat.Requests
			stat.AvgFirstResponseTimeMs = avgFirstResponseTime(row.FirstResponseTimeMs, stat.Requests)
		}
		out = append(out, stat)
	}
	return out, nil
}

func (s *LogService) usageModelSeries(userGuid string, query UsageSummaryQuery, rankedModels []UsageDimensionStat, dates []DailyUsageData) ([]UsageNamedSeries, error) {
	modelNames := make([]string, 0, len(rankedModels))
	seen := map[string]bool{}
	for _, model := range rankedModels {
		modelName := fallbackName(model.ModelName, model.Name)
		if modelName == "" || modelName == "unknown" || seen[modelName] {
			continue
		}
		seen[modelName] = true
		modelNames = append(modelNames, modelName)
	}
	if len(modelNames) == 0 {
		return []UsageNamedSeries{}, nil
	}
	seriesByModel := map[string]map[string]*DailyUsageData{}
	db := s.usageRangeDB(userGuid, query).Where("model_name IN ?", modelNames)
	dateExpr := usageDateExprSQL(db)
	selectSQL := fmt.Sprintf(`
		%s as usage_date,
		model_name,
		COUNT(*) as requests,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 0 ELSE 1 END), 0) as errors,
		COALESCE(SUM(quota), 0) as quota,
		%s as cost,
		COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as completion_tokens
	`, dateExpr, usageCostSumSQL(db))
	var rows []usageModelDailyAggregateRow
	if err := db.Select(selectSQL).Group(dateExpr + ", model_name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ModelName == "" || row.Date == "" {
			continue
		}
		series := seriesByModel[row.ModelName]
		if series == nil {
			series = map[string]*DailyUsageData{}
			seriesByModel[row.ModelName] = series
		}
		series[row.Date] = &DailyUsageData{
			Date:     row.Date,
			Requests: row.Requests,
			Quota:    row.Quota,
			Cost:     row.Cost,
			Tokens:   row.PromptTokens + row.CompletionTokens,
			Success:  row.Success,
			Errors:   row.Errors,
		}
	}
	return buildModelSeries(seriesByModel, rankedModels, dates), nil
}

func (row usageDimensionAggregateRow) toUsageDimensionStat() UsageDimensionStat {
	return UsageDimensionStat{
		Name:                   fallbackName(row.Name, row.Key),
		UserGuid:               row.UserGuid,
		Username:               row.Username,
		TokenGuid:              row.TokenGuid,
		ProviderGuid:           row.ProviderGuid,
		ModelName:              row.ModelName,
		Requests:               row.Requests,
		Success:                row.Success,
		Errors:                 row.Errors,
		Quota:                  row.Quota,
		Cost:                   row.Cost,
		PromptTokens:           row.PromptTokens,
		CompletionTokens:       row.CompletionTokens,
		Tokens:                 row.PromptTokens + row.CompletionTokens,
		AvgUseTimeMs:           row.UseTimeMs,
		AvgFirstResponseTimeMs: row.FirstResponseTimeMs,
	}
}

func applyAggregateToSummary(summary *UsageSummary, row usageAggregateRow) {
	summary.TotalRequests = row.Requests
	summary.SuccessRequests = row.Success
	summary.ErrorRequests = row.Errors
	summary.Quota = row.Quota
	summary.Cost = row.Cost
	summary.PromptTokens = row.PromptTokens
	summary.CompletionTokens = row.CompletionTokens
	summary.Tokens = row.PromptTokens + row.CompletionTokens
	summary.StreamRequests = row.StreamRequests
	if row.Requests > 0 {
		summary.AvgUseTimeMs = row.UseTimeMs / row.Requests
		summary.AvgFirstResponseTimeMs = avgFirstResponseTime(row.FirstResponseTimeMs, row.Requests)
	}
}

func avgFirstResponseTime(total int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return total / count
}

func usageFirstResponseTimeSumSQL() string {
	// Old logs may not have first-response latency; fall back to total duration for useful history charts.
	return "COALESCE(SUM(CASE WHEN first_response_time_ms > 0 THEN first_response_time_ms ELSE use_time_ms END), 0)"
}

func (s *LogService) enrichUsageUsers(rows []UsageDimensionStat) {
	userGuids := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if row.UserGuid == "" || seen[row.UserGuid] {
			continue
		}
		seen[row.UserGuid] = true
		userGuids = append(userGuids, row.UserGuid)
	}
	if len(userGuids) == 0 {
		return
	}
	var users []commonDomains.SysUser
	if err := s.DB().Where("guid IN ?", userGuids).Find(&users).Error; err != nil {
		return
	}
	userByGuid := make(map[string]commonDomains.SysUser, len(users))
	for _, user := range users {
		userByGuid[user.Guid] = user
	}
	for i := range rows {
		user, ok := userByGuid[rows[i].UserGuid]
		if !ok {
			continue
		}
		if user.Username != "" {
			rows[i].Username = user.Username
			rows[i].Name = user.Username
		}
		if user.Email != "" {
			rows[i].Email = user.Email
		}
	}
}

func normalizeUsageSummaryQuery(query UsageSummaryQuery) UsageSummaryQuery {
	days := query.Days
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	topN := query.TopN
	if topN <= 0 {
		topN = 10
	}
	if topN > 50 {
		topN = 50
	}
	endTime := normalizeUsageQueryTime(query.EndTime)
	startTime := normalizeUsageQueryTime(query.StartTime)
	if startTime <= 0 && endTime <= 0 {
		end := time.Now()
		start := beginningOfDay(end).AddDate(0, 0, -(days - 1))
		return UsageSummaryQuery{Days: days, TopN: topN, StartTime: start.UnixMilli(), EndTime: end.UnixMilli()}
	}
	if endTime <= 0 {
		endTime = time.Now().UnixMilli()
	}
	if startTime <= 0 {
		startTime = beginningOfDay(time.UnixMilli(endTime)).AddDate(0, 0, -(days - 1)).UnixMilli()
	}
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}
	startDay := beginningOfDay(time.UnixMilli(startTime))
	endDay := beginningOfDay(time.UnixMilli(endTime))
	days = int(endDay.Sub(startDay).Hours()/24) + 1
	if days <= 0 {
		days = 1
	}
	if days > 366 {
		days = 366
		startTime = endDay.AddDate(0, 0, -(days - 1)).UnixMilli()
	}
	return UsageSummaryQuery{Days: days, TopN: topN, StartTime: startTime, EndTime: endTime}
}

func buildUsageDayWindows(startTime int64, endTime int64) []usageDayWindow {
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}
	start := beginningOfDay(time.UnixMilli(startTime))
	endDay := beginningOfDay(time.UnixMilli(endTime))
	days := int(endDay.Sub(start).Hours()/24) + 1
	if days <= 0 {
		days = 1
	}
	if days > 366 {
		days = 366
		start = endDay.AddDate(0, 0, -(days - 1))
		startTime = start.UnixMilli()
	}
	windows := make([]usageDayWindow, 0, days)
	for i := 0; i < days; i++ {
		dayStart := start.AddDate(0, 0, i)
		windows = append(windows, usageDayWindow{
			Date: dayStart.Format("2006-01-02"),
		})
	}
	return windows
}

func buildDailyUsageSeries(userGuid string, windows []usageDayWindow, byDate map[string]DailyUsageData) []DailyUsageData {
	out := make([]DailyUsageData, 0, len(windows))
	for _, window := range windows {
		if item, ok := byDate[window.Date]; ok {
			item.UserGuid = userGuid
			out = append(out, item)
			continue
		}
		out = append(out, DailyUsageData{Date: window.Date, UserGuid: userGuid})
	}
	return out
}

func usageCostSumSQL(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "COALESCE(SUM(CASE WHEN COALESCE(cost, 0) > 0 THEN cost WHEN TRIM(COALESCE(other, '')) <> '' AND json_valid(other) THEN COALESCE(CAST(json_extract(other, '$.finalCost') AS REAL), 0) ELSE 0 END), 0)"
	case "mysql":
		return "COALESCE(SUM(CASE WHEN COALESCE(cost, 0) > 0 THEN cost WHEN TRIM(COALESCE(other, '')) <> '' AND JSON_VALID(other) THEN COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(other, '$.finalCost')) AS DECIMAL(20,10)), 0) ELSE 0 END), 0)"
	case "postgres":
		return "COALESCE(SUM(CASE WHEN COALESCE(cost, 0) > 0 THEN cost WHEN btrim(COALESCE(other, '')) <> '' THEN COALESCE((other::jsonb ->> 'finalCost')::double precision, 0) ELSE 0 END), 0)"
	default:
		return "COALESCE(SUM(cost), 0)"
	}
}

func usageDateExprSQL(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "DATE_FORMAT(FROM_UNIXTIME(create_time / 1000), '%Y-%m-%d')"
	case "postgres":
		return "TO_CHAR(TO_TIMESTAMP(create_time / 1000.0), 'YYYY-MM-DD')"
	default:
		return "DATE(create_time / 1000, 'unixepoch', 'localtime')"
	}
}

func buildModelSeries(seriesByModel map[string]map[string]*DailyUsageData, rankedModels []UsageDimensionStat, dates []DailyUsageData) []UsageNamedSeries {
	out := make([]UsageNamedSeries, 0, len(rankedModels))
	for _, model := range rankedModels {
		key := fallbackName(model.ModelName, model.Name)
		series := seriesByModel[key]
		if series == nil {
			series = seriesByModel[model.Name]
		}
		points := make([]DailyUsageData, 0, len(dates))
		for _, dateItem := range dates {
			date := dateItem.Date
			if point, ok := series[date]; ok {
				points = append(points, DailyUsageData{
					Date:     date,
					Requests: point.Requests,
					Quota:    point.Quota,
					Cost:     point.Cost,
					Tokens:   point.Tokens,
					Success:  point.Success,
					Errors:   point.Errors,
				})
				continue
			}
			points = append(points, DailyUsageData{Date: date})
		}
		out = append(out, UsageNamedSeries{
			Name:      fallbackName(model.Name, key),
			ModelName: fallbackName(model.ModelName, model.Name),
			Data:      points,
		})
	}
	return out
}

func fallbackName(primary string, fallback string) string {
	if primary != "" {
		return primary
	}
	if fallback != "" {
		return fallback
	}
	return "unknown"
}

func beginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
