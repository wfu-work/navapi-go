package services

import (
	"testing"
	"time"

	"navapi-go/domains"
	"navapi-go/dto"

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

	result, err := LogServiceApp.List("user-a", dto.PageQuery{Page: 1, Size: 10})
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
