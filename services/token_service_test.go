package services

import (
	"strings"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
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
		UserGuid:            "user-a",
		Name:                "client-token",
		Key:                 "sk-client",
		Status:              constants.StatusEnabled,
		Group:               constants.DefaultGroup,
		BalanceAmountMicros: 100,
		UnlimitedBalance:    false,
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

func TestTokenListIncludesUserProfile(t *testing.T) {
	db := withTokenTestDB(t)
	user := commonDomains.SysUser{
		Username: "alice",
		Email:    "alice@example.com",
	}
	user.Guid = "user-alice"
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token := domains.ApiToken{
		UserGuid:            user.Guid,
		Name:                "alice-token",
		Key:                 "sk-alice",
		Status:              constants.StatusEnabled,
		Group:               constants.DefaultGroup,
		BalanceAmountMicros: 100,
		UnlimitedBalance:    false,
	}
	token.Guid = "token-alice"
	if err := db.Create(&token).Error; err != nil {
		t.Fatal(err)
	}

	tokens, err := TokenServiceApp.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 {
		t.Fatalf("tokens len = %d, want 1", len(tokens))
	}
	if tokens[0].Username != user.Username || tokens[0].Email != user.Email {
		t.Fatalf("token user = %q/%q, want %q/%q", tokens[0].Username, tokens[0].Email, user.Username, user.Email)
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
	if err := db.AutoMigrate(&domains.ApiToken{}, &domains.UsageLog{}, &domains.UserQuota{}, &domains.Option{}, &domains.ModelMeta{}, &domains.ModelGroup{}, &commonDomains.SysUser{}); err != nil {
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
