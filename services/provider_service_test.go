package services

import (
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProviderSaveNormalizesAndPreservesKeyOnUpdate(t *testing.T) {
	withProviderTestDB(t)

	provider := domains.VendorMeta{
		VendorName:          " deepseek ",
		DisplayName:         " DeepSeek ",
		Key:                 " sk-provider ",
		Models:              " deepseek-chat\ndeepseek-reasoner, deepseek-chat ",
		ModelOverride:       " deepseek-fixed ",
		QuotaModelWhitelist: " deepseek-chat\ndeepseek-reasoner ",
		BalanceTemplate:     "unknown",
		Enabled:             true,
	}
	if err := ProviderServiceApp.Save(&provider); err != nil {
		t.Fatal(err)
	}
	if provider.Id == 0 {
		t.Fatal("provider id was not set")
	}
	if provider.Guid == "" {
		t.Fatal("provider guid was not set")
	}
	if provider.VendorName != "deepseek" || provider.DisplayName != "DeepSeek" {
		t.Fatalf("provider names were not normalized: %+v", provider)
	}
	if provider.Type != constants.ProviderTypeOpenAI {
		t.Fatalf("type = %q, want openai", provider.Type)
	}
	if provider.Models != "deepseek-chat,deepseek-reasoner,deepseek-chat" {
		t.Fatalf("models = %q", provider.Models)
	}
	if provider.ModelOverride != "deepseek-fixed" {
		t.Fatalf("modelOverride = %q, want normalized override", provider.ModelOverride)
	}
	if provider.QuotaModelWhitelist != "deepseek-chat,deepseek-reasoner" {
		t.Fatalf("quotaModelWhitelist = %q, want normalized list", provider.QuotaModelWhitelist)
	}
	if provider.BalanceTemplate != "generic" || provider.BalanceCustomPath != "/v1/usage" || provider.BalanceAuthType != "provider_bearer" || provider.BalanceRemainingPath != "remaining" || provider.BalanceMultiplier != 1 || provider.BalanceUnit != "USD" {
		t.Fatalf("balance defaults were not normalized: %+v", provider)
	}

	update := domains.VendorMeta{
		VendorName:  "deepseek",
		DisplayName: "DeepSeek API",
		Type:        constants.ProviderTypeOpenAI,
		Enabled:     true,
	}
	update.Guid = provider.Guid
	if err := ProviderServiceApp.Save(&update); err != nil {
		t.Fatal(err)
	}

	reloaded, err := ProviderServiceApp.GetByID(provider.Id)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DisplayName != "DeepSeek API" {
		t.Fatalf("displayName = %q, want updated value", reloaded.DisplayName)
	}
	if reloaded.Key != "sk-provider" {
		t.Fatalf("key = %q, want preserved key", reloaded.Key)
	}

	result, err := ProviderServiceApp.List(ProviderListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	records := result.List.([]ProviderRecord)
	if len(records) != 1 || !records[0].HasKey {
		t.Fatalf("records = %+v, want one record with hasKey", records)
	}
}

func TestProviderListFilters(t *testing.T) {
	withProviderTestDB(t)

	providers := []domains.VendorMeta{
		{VendorName: "openai", DisplayName: "OpenAI", Type: constants.ProviderTypeOpenAI, Key: "sk-openai", Enabled: true},
		{VendorName: "gemini", DisplayName: "Gemini", Type: constants.ProviderTypeGemini, Enabled: true},
		{VendorName: "old", DisplayName: "Old Provider", Type: constants.ProviderTypeOpenAI, Key: "sk-old", Enabled: false},
	}
	for i := range providers {
		if err := ProviderServiceApp.Save(&providers[i]); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ProviderServiceApp.List(ProviderListQuery{Type: constants.ProviderTypeOpenAI, Status: "enabled", KeyStatus: "set"})
	if err != nil {
		t.Fatal(err)
	}
	records := result.List.([]ProviderRecord)
	if result.Total != 1 || len(records) != 1 || records[0].VendorName != "openai" {
		t.Fatalf("filtered records = %+v total=%d, want only openai", records, result.Total)
	}

	result, err = ProviderServiceApp.List(ProviderListQuery{KeyStatus: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	records = result.List.([]ProviderRecord)
	if result.Total != 1 || len(records) != 1 || records[0].VendorName != "gemini" {
		t.Fatalf("missing-key records = %+v total=%d, want only gemini", records, result.Total)
	}
}

func TestProviderRoutingUsesEnabledKeyedProviders(t *testing.T) {
	withProviderTestDB(t)

	disabled := domains.VendorMeta{
		VendorName:  "disabled",
		DisplayName: "Disabled",
		Type:        constants.ProviderTypeOpenAI,
		Key:         "sk-disabled",
		Enabled:     false,
	}
	if err := ProviderServiceApp.Save(&disabled); err != nil {
		t.Fatal(err)
	}

	noKey := domains.VendorMeta{
		VendorName:  "nokey",
		DisplayName: "No Key",
		Type:        constants.ProviderTypeOpenAI,
		Enabled:     true,
	}
	if err := ProviderServiceApp.Save(&noKey); err != nil {
		t.Fatal(err)
	}

	enabled := domains.VendorMeta{
		VendorName:           "openai",
		DisplayName:          "OpenAI",
		Type:                 constants.ProviderTypeOpenAI,
		BaseURL:              "https://api.openai.com/v1",
		Key:                  "sk-provider",
		Models:               "gpt-4o-mini",
		ModelOverride:        "gpt-fixed",
		QuotaModelWhitelist:  "gpt-4o-mini",
		ModelMapping:         `{"public":"upstream"}`,
		HeaderOverride:       `{"X-Test":"yes"}`,
		BalanceCheckEnabled:  true,
		BalanceTemplate:      "custom",
		BalanceBaseURL:       "https://billing.example.com",
		BalanceAccessToken:   "balance-token",
		BalanceUserID:        "user-1",
		BalanceCustomPath:    "/quota",
		BalanceAuthType:      "provider_bearer",
		BalanceRemainingPath: "data.remaining",
		BalanceMultiplier:    100,
		BalanceUnit:          "credits",
		BalanceTotalPath:     "data.total",
		BalanceUsedPath:      "data.used",
		BalancePlanPath:      "data.plan",
		BalanceValidPath:     "data.active",
		BalanceErrorPath:     "message",
		Enabled:              true,
	}
	if err := ProviderServiceApp.Save(&enabled); err != nil {
		t.Fatal(err)
	}
	candidates, err := ProviderServiceApp.FindCandidatesForModelAndType("gpt-4o-mini", constants.DefaultGroup, constants.ProviderTypeOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].VendorName != "openai" {
		t.Fatalf("candidates = %+v, want only enabled keyed provider", candidates)
	}
	if mapped := ProviderServiceApp.MapModel(&candidates[0], "public"); mapped != "gpt-fixed" {
		t.Fatalf("mapped model = %q, want fixed override", mapped)
	}
	candidates[0].ModelOverride = ""
	if mapped := ProviderServiceApp.MapModel(&candidates[0], "public"); mapped != "upstream" {
		t.Fatalf("mapped model = %q, want mapped upstream model", mapped)
	}
}

func withProviderTestDB(t *testing.T) {
	t.Helper()
	previous := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.VendorMeta{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previous
	})
}
