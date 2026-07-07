package services

import (
	"testing"

	"navapi-go/domains"
	"navapi-go/vos"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserWalletEnsureWithInitialAmount(t *testing.T) {
	db := withUserWalletTestDB(t)
	if err := UserWalletServiceApp.EnsureWithInitialAmount(db, "user-001", 80); err != nil {
		t.Fatal(err)
	}

	wallet, err := UserWalletServiceApp.Get("user-001")
	if err != nil {
		t.Fatal(err)
	}
	if wallet.BalanceAmountMicros != 80 ||
		wallet.PaidBalanceAmountMicros != 80 ||
		wallet.TotalConsumedAmountMicros != 0 ||
		wallet.TotalRechargeAmountMicros != 80 {
		t.Fatalf("wallet = %+v, want initial amount totals", wallet)
	}
}

func TestUserWalletRecordsIncomeAndConsume(t *testing.T) {
	withUserWalletTestDB(t)

	err := UserWalletServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid:     "user-002",
			Type:         domains.WalletRecordTypeRecharge,
			Source:       domains.WalletSourcePayment,
			Title:        "充值",
			AmountMicros: 100,
		}); err != nil {
			return err
		}
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid:     "user-002",
			Type:         domains.WalletRecordTypeReward,
			Source:       domains.WalletSourceInvitation,
			Title:        "奖励",
			AmountMicros: 30,
		}); err != nil {
			return err
		}
		return UserWalletServiceApp.RecordConsume(tx, WalletRecordInput{
			UserGuid:     "user-002",
			AmountMicros: 40,
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
	if wallet.BalanceAmountMicros != 90 ||
		wallet.PaidBalanceAmountMicros != 90 ||
		wallet.RewardBalanceAmountMicros != 0 ||
		wallet.TotalRechargeAmountMicros != 100 ||
		wallet.TotalRewardAmountMicros != 30 ||
		wallet.TotalConsumedAmountMicros != 40 ||
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
	if records[2].Type != domains.WalletRecordTypeConsume || records[2].AmountMicrosDelta != -40 || records[2].BalanceAmountMicrosAfter != 90 {
		t.Fatalf("consume record = %+v, want amount -40 balance 90", records[2])
	}
}

func TestUserWalletListRecordsScopesByUserGuid(t *testing.T) {
	withUserWalletTestDB(t)
	err := UserWalletServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid:     "user-a",
			Type:         domains.WalletRecordTypeRecharge,
			Source:       domains.WalletSourcePayment,
			Title:        "充值",
			AmountMicros: 100,
		}); err != nil {
			return err
		}
		return UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid:     "user-b",
			Type:         domains.WalletRecordTypeReward,
			Source:       domains.WalletSourceInvitation,
			Title:        "奖励",
			AmountMicros: 50,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := UserWalletServiceApp.ListRecords("user-a", vos.PageQuery{Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want user-a record only", result.Total)
	}
	records := result.List.([]domains.UserWalletRecord)
	if len(records) != 1 || records[0].UserGuid != "user-a" {
		t.Fatalf("records = %+v, want user-a only", records)
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
