package services

import (
	"encoding/json"
	"sort"
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
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Quota    int64  `json:"quota"`
	Tokens   int64  `json:"tokens"`
	Success  int64  `json:"success"`
	Errors   int64  `json:"errors"`
	UserGuid string `json:"userGuid,omitempty"`
}

type UsageNamedSeries struct {
	Name      string           `json:"name"`
	ModelName string           `json:"modelName,omitempty"`
	Data      []DailyUsageData `json:"data"`
}

type UsageDimensionStat struct {
	Name             string `json:"name"`
	UserGuid         string `json:"userGuid,omitempty"`
	Username         string `json:"username,omitempty"`
	Email            string `json:"email,omitempty"`
	TokenGuid        string `json:"tokenGuid,omitempty"`
	ProviderGuid     string `json:"providerGuid,omitempty"`
	ModelName        string `json:"modelName,omitempty"`
	Requests         int64  `json:"requests"`
	Success          int64  `json:"success"`
	Errors           int64  `json:"errors"`
	Quota            int64  `json:"quota"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
	Tokens           int64  `json:"tokens"`
	AvgUseTimeMs     int64  `json:"avgUseTimeMs"`
}

type UsageSummary struct {
	Days             int                  `json:"days"`
	StartTime        int64                `json:"startTime,omitempty"`
	EndTime          int64                `json:"endTime,omitempty"`
	TotalRequests    int64                `json:"totalRequests"`
	SuccessRequests  int64                `json:"successRequests"`
	ErrorRequests    int64                `json:"errorRequests"`
	Quota            int64                `json:"quota"`
	PromptTokens     int64                `json:"promptTokens"`
	CompletionTokens int64                `json:"completionTokens"`
	Tokens           int64                `json:"tokens"`
	AvgUseTimeMs     int64                `json:"avgUseTimeMs"`
	StreamRequests   int64                `json:"streamRequests"`
	Series           []DailyUsageData     `json:"series"`
	SeriesByModel    []UsageNamedSeries   `json:"seriesByModel"`
	ByModel          []UsageDimensionStat `json:"byModel"`
	ByProvider       []UsageDimensionStat `json:"byProvider"`
	ByToken          []UsageDimensionStat `json:"byToken"`
	ByUser           []UsageDimensionStat `json:"byUser,omitempty"`
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

func (s *LogService) Create(log *domains.UsageLog) error {
	return createWithCrud(&s.CrudService, log)
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
	s.enrichOfficialCosts(logs)
	return vos.PageResult{List: logs, Total: total, Page: query.Page, Size: query.Size}, nil
}

func applyUsageLogFilters(db *gorm.DB, userGuid string, query UsageLogQuery) *gorm.DB {
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		db = db.Where("model_name LIKE ? OR token_name LIKE ? OR channel_name LIKE ? OR user_guid LIKE ? OR username LIKE ? OR request_id LIKE ? OR upstream_request_id LIKE ?", keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
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

func normalizeUsageQueryTime(value int64) int64 {
	if value > 0 && value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

func (s *LogService) enrichOfficialCosts(logs []domains.UsageLog) {
	for i := range logs {
		extra := usageLogExtraMap(logs[i].Other)
		if _, ok := extra["finalCost"]; ok {
			continue
		}
		group := normalizeGroup(extraText(extra["group"]))
		usage := vos.Usage{
			PromptTokens:     logs[i].PromptTokens,
			CompletionTokens: logs[i].CompletionTokens,
			CachedTokens:     int64(extraNumber(extra["cachedTokens"])),
		}
		detail := PricingServiceApp.WithDB(s.DB()).OfficialCostDetail(logs[i].ModelName, group, usage)
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
		data, err := json.Marshal(extra)
		if err == nil {
			logs[i].Other = string(data)
		}
	}
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
	} else if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	var totalRequests int64
	if err := db.Count(&totalRequests).Error; err != nil {
		return nil, err
	}
	var sums struct {
		Quota            int64
		PromptTokens     int64
		CompletionTokens int64
		UseTimeMs        int64
	}
	if err := db.Select("COALESCE(SUM(quota),0) as quota, COALESCE(SUM(prompt_tokens),0) as prompt_tokens, COALESCE(SUM(completion_tokens),0) as completion_tokens, COALESCE(SUM(use_time_ms),0) as use_time_ms").Scan(&sums).Error; err != nil {
		return nil, err
	}
	var successCount int64
	if err := db.Session(&gorm.Session{}).Where("status = ?", "success").Count(&successCount).Error; err != nil {
		return nil, err
	}
	avgUseTimeMs := int64(0)
	if totalRequests > 0 {
		avgUseTimeMs = sums.UseTimeMs / totalRequests
	}
	return map[string]any{
		"totalRequests":    totalRequests,
		"successRequests":  successCount,
		"errorRequests":    totalRequests - successCount,
		"quota":            sums.Quota,
		"promptTokens":     sums.PromptTokens,
		"completionTokens": sums.CompletionTokens,
		"tokens":           sums.PromptTokens + sums.CompletionTokens,
		"avgUseTimeMs":     avgUseTimeMs,
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
	db := s.DB().Model(&domains.UsageLog{}).Where("create_time >= ? AND create_time <= ?", startTime, endTime)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	var logs []domains.UsageLog
	if err := db.Select("create_time", "quota", "prompt_tokens", "completion_tokens", "status").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	byDate := map[string]DailyUsageData{}
	for _, log := range logs {
		date := time.UnixMilli(log.CreateTime).Format("2006-01-02")
		item := byDate[date]
		item.Date = date
		item.UserGuid = userGuid
		item.Requests++
		item.Quota += log.Quota
		item.Tokens += log.PromptTokens + log.CompletionTokens
		if log.Status == "success" {
			item.Success++
		} else {
			item.Errors++
		}
		byDate[date] = item
	}
	out := make([]DailyUsageData, 0, days)
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		if item, ok := byDate[date]; ok {
			out = append(out, item)
			continue
		}
		out = append(out, DailyUsageData{Date: date, UserGuid: userGuid})
	}
	return out, nil
}

// UsageSummary builds dashboard-ready aggregates without relying on
// database-specific date functions, keeping the statistics portable.
func (s *LogService) UsageSummary(userGuid string, days int, topN int) (UsageSummary, error) {
	return s.UsageSummaryByQuery(userGuid, UsageSummaryQuery{Days: days, TopN: topN})
}

func (s *LogService) UsageSummaryByQuery(userGuid string, query UsageSummaryQuery) (UsageSummary, error) {
	normalized := normalizeUsageSummaryQuery(query)
	series, err := s.dailyDataInRange(userGuid, normalized.StartTime, normalized.EndTime)
	if err != nil {
		return UsageSummary{}, err
	}
	db := s.DB().Model(&domains.UsageLog{}).Where("create_time >= ? AND create_time <= ?", normalized.StartTime, normalized.EndTime)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	var logs []domains.UsageLog
	if err := db.Select("create_time", "user_guid", "username", "token_guid", "token_name", "channel_guid", "channel_name", "model_name", "quota", "prompt_tokens", "completion_tokens", "use_time_ms", "is_stream", "status").
		Find(&logs).Error; err != nil {
		return UsageSummary{}, err
	}
	summary := UsageSummary{Days: normalized.Days, StartTime: normalized.StartTime, EndTime: normalized.EndTime, Series: series}
	byModel := map[string]*UsageDimensionStat{}
	byProvider := map[string]*UsageDimensionStat{}
	byToken := map[string]*UsageDimensionStat{}
	byUser := map[string]*UsageDimensionStat{}
	byModelSeries := map[string]map[string]*DailyUsageData{}
	for _, log := range logs {
		applyUsageStat(&summary, log)
		modelKey := fallbackName(log.ModelName, "unknown")
		modelName := fallbackName(log.ModelName, "unknown")
		applyDimensionStat(byModel, modelKey, modelName, log, func(item *UsageDimensionStat, log domains.UsageLog) {
			fillUsageDimensionText(&item.ModelName, log.ModelName)
		})
		applyModelSeriesStat(byModelSeries, modelKey, modelName, log)
		applyDimensionStat(byProvider, fallbackName(log.ProviderGuid, log.ProviderName), fallbackName(log.ProviderName, log.ProviderGuid), log, func(item *UsageDimensionStat, log domains.UsageLog) {
			fillUsageDimensionText(&item.ProviderGuid, log.ProviderGuid)
		})
		applyDimensionStat(byToken, fallbackName(log.TokenGuid, log.TokenName), fallbackName(log.TokenName, log.TokenGuid), log, func(item *UsageDimensionStat, log domains.UsageLog) {
			fillUsageDimensionText(&item.TokenGuid, log.TokenGuid)
			fillUsageDimensionText(&item.UserGuid, log.UserGuid)
		})
		if userGuid == "" {
			applyDimensionStat(byUser, fallbackName(log.UserGuid, log.Username), fallbackName(log.Username, log.UserGuid), log, func(item *UsageDimensionStat, log domains.UsageLog) {
				fillUsageDimensionText(&item.UserGuid, log.UserGuid)
				fillUsageDimensionText(&item.Username, log.Username)
			})
		}
	}
	if summary.TotalRequests > 0 {
		summary.AvgUseTimeMs = summary.AvgUseTimeMs / summary.TotalRequests
	}
	summary.ByModel = topUsageStats(byModel, normalized.TopN)
	summary.SeriesByModel = buildModelSeries(byModelSeries, summary.ByModel, series)
	summary.ByProvider = topUsageStats(byProvider, normalized.TopN)
	summary.ByToken = topUsageStats(byToken, normalized.TopN)
	if userGuid == "" {
		summary.ByUser = topUsageStats(byUser, normalized.TopN)
		s.enrichUsageUsers(summary.ByUser)
	}
	return summary, nil
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

func applyUsageStat(summary *UsageSummary, log domains.UsageLog) {
	summary.TotalRequests++
	if log.Status == "success" {
		summary.SuccessRequests++
	} else {
		summary.ErrorRequests++
	}
	if log.IsStream {
		summary.StreamRequests++
	}
	summary.Quota += log.Quota
	summary.PromptTokens += log.PromptTokens
	summary.CompletionTokens += log.CompletionTokens
	summary.Tokens += log.PromptTokens + log.CompletionTokens
	summary.AvgUseTimeMs += log.UseTimeMs
}

func applyDimensionStat(items map[string]*UsageDimensionStat, key string, name string, log domains.UsageLog, decorate func(*UsageDimensionStat, domains.UsageLog)) {
	key = fallbackName(key, name)
	name = fallbackName(name, key)
	item := items[key]
	if item == nil {
		item = &UsageDimensionStat{Name: name}
		items[key] = item
	}
	if decorate != nil {
		decorate(item, log)
	}
	item.Requests++
	if log.Status == "success" {
		item.Success++
	} else {
		item.Errors++
	}
	item.Quota += log.Quota
	item.PromptTokens += log.PromptTokens
	item.CompletionTokens += log.CompletionTokens
	item.Tokens += log.PromptTokens + log.CompletionTokens
	item.AvgUseTimeMs += log.UseTimeMs
}

func applyModelSeriesStat(items map[string]map[string]*DailyUsageData, key string, name string, log domains.UsageLog) {
	key = fallbackName(key, name)
	date := time.UnixMilli(log.CreateTime).Format("2006-01-02")
	series := items[key]
	if series == nil {
		series = map[string]*DailyUsageData{}
		items[key] = series
	}
	item := series[date]
	if item == nil {
		item = &DailyUsageData{Date: date}
		series[date] = item
	}
	item.Requests++
	item.Quota += log.Quota
	item.Tokens += log.PromptTokens + log.CompletionTokens
	if log.Status == "success" {
		item.Success++
	} else {
		item.Errors++
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

func fillUsageDimensionText(target *string, value string) {
	if *target == "" && value != "" {
		*target = value
	}
}

func topUsageStats(items map[string]*UsageDimensionStat, limit int) []UsageDimensionStat {
	out := make([]UsageDimensionStat, 0, len(items))
	for _, item := range items {
		if item.Requests > 0 {
			item.AvgUseTimeMs = item.AvgUseTimeMs / item.Requests
		}
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Quota == out[j].Quota {
			return out[i].Requests > out[j].Requests
		}
		return out[i].Quota > out[j].Quota
	})
	if len(out) > limit {
		return out[:limit]
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
