package services

import (
	"testing"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserWalletEnsureMirrorsExistingQuota(t *testing.T) {
	db := withUserWalletTestDB(t)
	if err := db.Create(&domains.UserQuota{
		UserGuid:    "user-001",
		RemainQuota: 80,
		UsedQuota:   20,
		TotalQuota:  100,
	}).Error; err != nil {
		t.Fatal(err)
	}

	wallet, err := UserWalletServiceApp.Get("user-001")
	if err != nil {
		t.Fatal(err)
	}
	if wallet.BalanceQuota != 80 || wallet.PaidBalanceQuota != 80 || wallet.TotalConsumedQuota != 20 || wallet.TotalRechargeQuota != 100 {
		t.Fatalf("wallet = %+v, want mirrored quota totals", wallet)
	}
}

func TestUserWalletRecordsIncomeAndConsume(t *testing.T) {
	withUserWalletTestDB(t)

	err := UserWalletServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid: "user-002",
			Type:     domains.WalletRecordTypeRecharge,
			Source:   domains.WalletSourcePayment,
			Title:    "充值",
			Quota:    100,
		}); err != nil {
			return err
		}
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid: "user-002",
			Type:     domains.WalletRecordTypeReward,
			Source:   domains.WalletSourceInvitation,
			Title:    "奖励",
			Quota:    30,
		}); err != nil {
			return err
		}
		return UserWalletServiceApp.RecordConsume(tx, WalletRecordInput{
			UserGuid:     "user-002",
			Quota:        40,
			RequestCount: 2,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	wallet, err := UserWalletServiceApp.Get("user-002")
	if err != nil {
		t.Fatal(err)
	}
	if wallet.BalanceQuota != 90 ||
		wallet.PaidBalanceQuota != 90 ||
		wallet.RewardBalanceQuota != 0 ||
		wallet.TotalRechargeQuota != 100 ||
		wallet.TotalRewardQuota != 30 ||
		wallet.TotalConsumedQuota != 40 ||
		wallet.TotalRequestCount != 2 {
		t.Fatalf("wallet = %+v, want income and consumption totals", wallet)
	}
	var records []domains.UserWalletRecord
	if err := UserWalletServiceApp.RecordCrud.DB().Order("id").Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records len = %d, want 3", len(records))
	}
	if records[2].Type != domains.WalletRecordTypeConsume || records[2].QuotaDelta != -40 || records[2].BalanceAfter != 90 {
		t.Fatalf("consume record = %+v, want -40 balance 90", records[2])
	}
}

func withUserWalletTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.UserWallet{}, &domains.UserWalletRecord{}, &domains.UserQuota{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previousDB
	})
	return db
}
