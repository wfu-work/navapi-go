package services

import (
	"testing"
	"time"

	"navapi-go/domains"
	"navapi-go/vos"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLogServiceScopesSelfQueriesByUserGuid(t *testing.T) {
	db := withLogTestDB(t)
	now := time.Now().UnixMilli()
	logs := []domains.UsageLog{
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now}, UserGuid: "user-a", TokenGuid: "token-a", TokenName: "A", ProviderName: "OpenAI", ModelName: "gpt-4o", Quota: 10, PromptTokens: 4, CompletionTokens: 6, Status: "success"},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now}, UserGuid: "user-b", TokenGuid: "token-b", TokenName: "B", ProviderName: "OpenAI", ModelName: "gpt-4o", Quota: 20, PromptTokens: 8, CompletionTokens: 12, Status: "error"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	result, err := LogServiceApp.List("user-a", UsageLogQuery{PageQuery: vos.PageQuery{Page: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	list := result.List.([]domains.UsageLog)
	if len(list) != 1 || list[0].UserGuid != "user-a" {
		t.Fatalf("list = %+v, want only user-a", list)
	}

	stats, err := LogServiceApp.Stats("user-a")
	if err != nil {
		t.Fatal(err)
	}
	if stats["totalRequests"] != int64(1) || stats["quota"] != int64(10) {
		t.Fatalf("stats = %+v, want user-a totals only", stats)
	}

	daily, err := LogServiceApp.DailyData("user-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 1 || daily[0].Requests != 1 || daily[0].Quota != 10 || daily[0].UserGuid != "user-a" {
		t.Fatalf("daily = %+v, want user-a day only", daily)
	}
}

func TestLogServiceFiltersUsageLogsByStatusAndTime(t *testing.T) {
	db := withLogTestDB(t)
	base := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC).UnixMilli()
	logs := []domains.UsageLog{
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: base - 3600_000}, UserGuid: "user-a", TokenName: "A", ModelName: "gpt-4o", Status: "success", Quota: 10, UseTimeMs: 100},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: base}, UserGuid: "user-a", TokenName: "A", ModelName: "gpt-4o", Status: "error", Quota: 20, UseTimeMs: 200},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: base + 3600_000}, UserGuid: "user-a", TokenName: "A", ModelName: "gpt-4o", Status: "success", Quota: 30, UseTimeMs: 300},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: base}, UserGuid: "user-b", TokenName: "B", ModelName: "gpt-4o", Status: "success", Quota: 40, UseTimeMs: 400},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	query := UsageLogQuery{
		PageQuery: vos.PageQuery{Page: 1, Size: 10},
		Status:    "success",
		StartTime: base - 1,
		EndTime:   base + 3600_000 + 1,
	}
	result, err := LogServiceApp.List("user-a", query)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want only matching user-a success log in time range", result.Total)
	}
	stats, err := LogServiceApp.Stats("user-a", query)
	if err != nil {
		t.Fatal(err)
	}
	if stats["totalRequests"] != int64(1) || stats["quota"] != int64(30) || stats["avgUseTimeMs"] != int64(300) {
		t.Fatalf("stats = %+v, want filtered success log totals", stats)
	}
}

func TestUsageSummaryKeepsUsersSeparateByGuid(t *testing.T) {
	db := withLogTestDB(t)
	now := time.Now().UnixMilli()
	logs := []domains.UsageLog{
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now}, UserGuid: "user-a", Username: "same-name", TokenGuid: "token-a", TokenName: "dev", ProviderGuid: "provider-1", ProviderName: "OpenAI", ModelName: "gpt-4o", Quota: 10, PromptTokens: 4, CompletionTokens: 6, Status: "success"},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now}, UserGuid: "user-b", Username: "same-name", TokenGuid: "token-b", TokenName: "dev", ProviderGuid: "provider-1", ProviderName: "OpenAI", ModelName: "gpt-4o", Quota: 20, PromptTokens: 8, CompletionTokens: 12, Status: "success"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := LogServiceApp.UsageSummary("", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 2 || summary.Quota != 30 {
		t.Fatalf("summary totals = %+v, want two records", summary)
	}
	if len(summary.ByUser) != 2 {
		t.Fatalf("byUser = %+v, want two users with same display name kept separate", summary.ByUser)
	}
	seen := map[string]bool{}
	for _, item := range summary.ByUser {
		seen[item.UserGuid] = true
	}
	if !seen["user-a"] || !seen["user-b"] {
		t.Fatalf("byUser = %+v, want userGuid fields for both users", summary.ByUser)
	}

	selfSummary, err := LogServiceApp.UsageSummary("user-a", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if selfSummary.TotalRequests != 1 || selfSummary.Quota != 10 || len(selfSummary.ByUser) != 0 {
		t.Fatalf("self summary = %+v, want user-a only without byUser", selfSummary)
	}
}

func TestUsageSummaryIncludesModelSeriesScopedByUser(t *testing.T) {
	db := withLogTestDB(t)
	now := time.Now()
	today := now.UnixMilli()
	yesterday := now.AddDate(0, 0, -1).UnixMilli()
	logs := []domains.UsageLog{
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: yesterday}, UserGuid: "user-a", ModelName: "gpt-5.5", Quota: 12, PromptTokens: 4, CompletionTokens: 8, Status: "success"},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: today}, UserGuid: "user-a", ModelName: "gpt-5.5", Quota: 18, PromptTokens: 6, CompletionTokens: 12, Status: "success"},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: today}, UserGuid: "user-a", ModelName: "gpt-5.4", Quota: 7, PromptTokens: 3, CompletionTokens: 4, Status: "error"},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: today}, UserGuid: "user-b", ModelName: "gpt-5.5", Quota: 99, PromptTokens: 40, CompletionTokens: 59, Status: "success"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := LogServiceApp.UsageSummary("user-a", 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 3 || summary.Quota != 37 {
		t.Fatalf("summary totals = %+v, want user-a model totals only", summary)
	}
	if len(summary.SeriesByModel) != 2 {
		t.Fatalf("seriesByModel = %+v, want two user-a model series", summary.SeriesByModel)
	}
	var gpt55 *UsageNamedSeries
	for i := range summary.SeriesByModel {
		if summary.SeriesByModel[i].ModelName == "gpt-5.5" {
			gpt55 = &summary.SeriesByModel[i]
			break
		}
	}
	if gpt55 == nil {
		t.Fatalf("seriesByModel = %+v, want gpt-5.5 series", summary.SeriesByModel)
	}
	if len(gpt55.Data) != 2 {
		t.Fatalf("gpt-5.5 series = %+v, want two date points", gpt55.Data)
	}
	if gpt55.Data[0].Quota != 12 || gpt55.Data[1].Quota != 18 {
		t.Fatalf("gpt-5.5 series = %+v, want user-a daily quota only", gpt55.Data)
	}
}

func withLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.UsageLog{}, &domains.ModelMeta{}, &domains.ModelGroup{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previousDB
	})
	return db
}
