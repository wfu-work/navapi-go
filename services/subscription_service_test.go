package services

import (
	"testing"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSubscriptionSavePlanGeneratesCodeFromName(t *testing.T) {
	previous := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.SubscriptionPlan{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previous
	})

	plan := domains.SubscriptionPlan{
		Name:         "Pro Month",
		WeeklyQuota:  -10,
		DurationDays: 30,
	}
	if err := SubscriptionServiceApp.SavePlan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan.Code != "pro-month" {
		t.Fatalf("code = %q, want pro-month", plan.Code)
	}
	if plan.WeeklyQuota != 0 {
		t.Fatalf("weekly quota = %d, want 0", plan.WeeklyQuota)
	}
}
