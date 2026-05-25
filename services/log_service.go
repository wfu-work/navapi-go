package services

import (
	"time"

	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
)

type LogService struct{}

var LogServiceApp = LogService{}

type DailyUsageData struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Quota    int64  `json:"quota"`
	Tokens   int64  `json:"tokens"`
	Success  int64  `json:"success"`
	Errors   int64  `json:"errors"`
	UserGuid string `json:"userGuid,omitempty"`
}

func (s LogService) Create(log *domains.UsageLog) error {
	return global.NAV_DB.Create(log).Error
}

func (s LogService) List(userGuid string, query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var logs []domains.UsageLog
	var total int64
	db := global.NAV_DB.Model(&domains.UsageLog{})
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		db = db.Where("model_name LIKE ? OR token_name LIKE ? OR channel_name LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&logs).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: logs, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s LogService) Stats(userGuid string) (map[string]any, error) {
	db := global.NAV_DB.Model(&domains.UsageLog{})
	if userGuid != "" {
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
	}
	if err := db.Select("COALESCE(SUM(quota),0) as quota, COALESCE(SUM(prompt_tokens),0) as prompt_tokens, COALESCE(SUM(completion_tokens),0) as completion_tokens").Scan(&sums).Error; err != nil {
		return nil, err
	}
	var successCount int64
	if err := db.Session(&gorm.Session{}).Where("status = ?", "success").Count(&successCount).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"totalRequests":    totalRequests,
		"successRequests":  successCount,
		"errorRequests":    totalRequests - successCount,
		"quota":            sums.Quota,
		"promptTokens":     sums.PromptTokens,
		"completionTokens": sums.CompletionTokens,
	}, nil
}

func (s LogService) DailyData(userGuid string, days int) ([]DailyUsageData, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	end := time.Now()
	start := beginningOfDay(end).AddDate(0, 0, -(days - 1))
	db := global.NAV_DB.Model(&domains.UsageLog{}).Where("create_time >= ?", start.UnixMilli())
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

func beginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
