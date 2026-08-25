package services

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"navapi-go/domains"
)

func TestAdminRechargeStatsSeparatesManualAndRedemptionIncome(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domains.UserWalletRecord{}); err != nil {
		t.Fatalf("migrate wallet records: %v", err)
	}

	records := []domains.UserWalletRecord{
		{
			Type:              domains.WalletRecordTypeRecharge,
			Direction:         domains.WalletRecordDirectionIncome,
			Source:            domains.WalletSourceManual,
			AmountMicrosDelta: 25_000_000,
		},
		{
			Type:        domains.WalletRecordTypeRecharge,
			Direction:   domains.WalletRecordDirectionIncome,
			Source:      domains.WalletSourceManual,
			AmountCents: 1_250,
		},
		{
			Type:              domains.WalletRecordTypeRecharge,
			Direction:         domains.WalletRecordDirectionIncome,
			Source:            domains.WalletSourceRedemption,
			AmountMicrosDelta: 100_000_000,
		},
		{
			Type:              domains.WalletRecordTypeRecharge,
			Direction:         domains.WalletRecordDirectionOutcome,
			Source:            domains.WalletSourceManual,
			AmountMicrosDelta: -5_000_000,
		},
		{
			Type:              domains.WalletRecordTypeSubscription,
			Direction:         domains.WalletRecordDirectionIncome,
			Source:            domains.WalletSourceManual,
			AmountMicrosDelta: 999_000_000,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create wallet records: %v", err)
	}

	service := UserWalletService{}
	service.RecordCrud = *service.RecordCrud.WithDB(db)
	stats, err := service.AdminRechargeStats()
	if err != nil {
		t.Fatalf("load recharge stats: %v", err)
	}
	if stats.ManualRechargeAmountMicros != 37_500_000 {
		t.Fatalf("manual total = %d, want %d", stats.ManualRechargeAmountMicros, int64(37_500_000))
	}
	if stats.RedemptionAmountMicros != 100_000_000 {
		t.Fatalf("redemption total = %d, want %d", stats.RedemptionAmountMicros, int64(100_000_000))
	}
}
