package services

import (
	"strings"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTokenCreateRequiresUserGuid(t *testing.T) {
	withTokenTestDB(t)

	err := TokenServiceApp.Create(&domains.ApiToken{Name: "missing user"})
	if err == nil || !strings.Contains(err.Error(), "user guid is required") {
		t.Fatalf("err = %v, want user guid validation", err)
	}
}

func TestTokenUsageFiltersLogsByUserGuid(t *testing.T) {
	db := withTokenTestDB(t)
	token := domains.ApiToken{
		UserGuid:       "user-a",
		Name:           "client-token",
		Key:            "sk-client",
		Status:         constants.StatusEnabled,
		Group:          constants.DefaultGroup,
		RemainQuota:    100,
		UnlimitedQuota: false,
	}
	token.Guid = "token-a"
	if err := db.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	logs := []domains.UsageLog{
		{UserGuid: "user-a", TokenGuid: "token-a", Quota: 10, PromptTokens: 4, CompletionTokens: 6, Status: "success"},
		{UserGuid: "user-b", TokenGuid: "token-a", Quota: 99, PromptTokens: 40, CompletionTokens: 59, Status: "success"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	usage, err := TokenServiceApp.Usage("user-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 1 {
		t.Fatalf("usage len = %d, want 1", len(usage))
	}
	if usage[0].LogQuota != 10 || usage[0].PromptTokens != 4 || usage[0].CompletionTokens != 6 {
		t.Fatalf("usage = %+v, want only user-a log totals", usage[0])
	}
}

func withTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	previousCache := OptionServiceApp.cache
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.ApiToken{}, &domains.UsageLog{}, &domains.UserQuota{}, &domains.Option{}, &domains.ModelMeta{}, &domains.ModelGroup{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	OptionServiceApp.cache = map[string]string{}
	if err := db.Create(&domains.ModelGroup{
		GroupName:       constants.DefaultGroup,
		DisplayName:     "Default",
		QuotaMultiplier: 1,
		Enabled:         true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		global.NAV_DB = previousDB
		OptionServiceApp.cache = previousCache
	})
	return db
}
