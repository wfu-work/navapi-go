package services

import (
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

func withPricingTestDB(t *testing.T) {
	t.Helper()
	previous := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.Pricing{}, &domains.ModelGroup{}); err != nil {
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
