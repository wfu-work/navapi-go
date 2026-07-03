package services

import (
	"testing"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserQuotaGetUsesDatabaseColumnName(t *testing.T) {
	withUserQuotaTestDB(t)

	account, err := UserQuotaServiceApp.Get("user-001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if account.UserGuid != "user-001" {
		t.Fatalf("UserGuid = %q, want user-001", account.UserGuid)
	}

	reloaded, err := UserQuotaServiceApp.Get("user-001")
	if err != nil {
		t.Fatalf("Get() reload error = %v", err)
	}
	if reloaded.Id != account.Id {
		t.Fatalf("reloaded Id = %d, want %d", reloaded.Id, account.Id)
	}
}

func withUserQuotaTestDB(t *testing.T) {
	t.Helper()
	previousDB := global.NAV_DB
	previousCache := OptionServiceApp.cache
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.UserQuota{}, &domains.Option{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	OptionServiceApp.cache = map[string]string{}
	t.Cleanup(func() {
		global.NAV_DB = previousDB
		OptionServiceApp.cache = previousCache
	})
}
