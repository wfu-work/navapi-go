package services

import (
	"math"
	"testing"

	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPricingCalculateQuotaAppliesModelGroupMultiplier(t *testing.T) {
	withPricingTestDB(t)

	group := domains.ModelGroup{
		GroupName:       "pro",
		DisplayName:     "专业版",
		QuotaMultiplier: 2,
		Enabled:         true,
	}
	if err := ModelServiceApp.UpsertGroup(&group); err != nil {
		t.Fatal(err)
	}
	pricing := domains.Pricing{
		ModelName:        "gpt-4o",
		Group:            "pro",
		PromptMultiplier: 1,
		OutputMultiplier: 3,
		CacheMultiplier:  1,
		QuotaMultiplier:  1,
		Enabled:          true,
	}
	if err := PricingServiceApp.Upsert(&pricing); err != nil {
		t.Fatal(err)
	}

	quota := PricingServiceApp.CalculateQuota("gpt-4o", "pro", dto.Usage{
		PromptTokens:     10,
		CompletionTokens: 10,
	}, 20)
	if quota != 80 {
		t.Fatalf("quota = %d, want 80", quota)
	}
}

func TestPricingOfficialCostAppliesModelGroupMultiplier(t *testing.T) {
	withPricingTestDB(t)

	group := domains.ModelGroup{
		GroupName:       "pro",
		DisplayName:     "专业版",
		QuotaMultiplier: 2,
		Enabled:         true,
	}
	if err := ModelServiceApp.UpsertGroup(&group); err != nil {
		t.Fatal(err)
	}
	model := domains.ModelMeta{
		ModelName:           "gpt-5.4-mini",
		Group:               "pro",
		Groups:              []string{"pro"},
		OfficialProvider:    "OpenAI",
		OfficialInputPrice:  3,
		OfficialOutputPrice: 15,
		OfficialCachePrice:  0.3,
		OfficialPriceUnit:   "1M tokens",
		Enabled:             true,
	}
	if err := ModelServiceApp.UpsertMeta(&model); err != nil {
		t.Fatal(err)
	}

	detail := PricingServiceApp.OfficialCostDetail("gpt-5.4-mini", "pro", dto.Usage{
		PromptTokens:     26,
		CompletionTokens: 24,
		CachedTokens:     6,
	})
	if !detail.OfficialPricing {
		t.Fatal("official pricing was not matched")
	}
	if detail.RegularPromptTokens != 20 || detail.CachedTokens != 6 || detail.CompletionTokens != 24 {
		t.Fatalf("tokens = regular:%d cached:%d output:%d", detail.RegularPromptTokens, detail.CachedTokens, detail.CompletionTokens)
	}
	if math.Abs(detail.RawCost-0.0004218) > 0.0000000001 {
		t.Fatalf("raw cost = %.10f, want 0.0004218", detail.RawCost)
	}
	if math.Abs(detail.FinalCost-0.0008436) > 0.0000000001 {
		t.Fatalf("final cost = %.10f, want 0.0008436", detail.FinalCost)
	}
}

func withPricingTestDB(t *testing.T) {
	t.Helper()
	previous := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.Pricing{}, &domains.ModelGroup{}, &domains.ModelMeta{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	if err := ModelServiceApp.EnsureDefaultGroup(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		global.NAV_DB = previous
	})
}
