package services

import (
	"strings"
	"testing"

	"navapi-go/domains"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConfirmWechatTransactionRechargesWallet(t *testing.T) {
	db := withWechatPaymentTestDB(t)
	order := createWechatPaymentOrder(t, 500, 200)
	transaction := &payments.Transaction{
		Appid:         core.String("wx-app"),
		Mchid:         core.String("mch-001"),
		OutTradeNo:    core.String(order.OrderNo),
		TradeState:    core.String("SUCCESS"),
		TradeType:     core.String(wechatTradeTypeNative),
		TransactionId: core.String("420000000000001"),
		Amount: &payments.TransactionAmount{
			Total:    core.Int64(500),
			Currency: core.String("CNY"),
		},
	}

	paid, err := PaymentServiceApp.ConfirmWechatTransaction(transaction, `{"trade_state":"SUCCESS"}`)
	if err != nil {
		t.Fatal(err)
	}
	if paid.Status != "paid" || paid.TransactionID != "420000000000001" {
		t.Fatalf("paid order = %+v, want paid with transaction id", paid)
	}

	var quota domains.UserQuota
	if err := db.Where("user_guid = ?", "user-wechat").First(&quota).Error; err != nil {
		t.Fatal(err)
	}
	if quota.RemainQuota != defaultRegisterQuota+200 || quota.TotalQuota != defaultRegisterQuota+200 {
		t.Fatalf("quota = %+v, want default quota plus recharge quota", quota)
	}

	var wallet domains.UserWallet
	if err := db.Where("user_guid = ?", "user-wechat").First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.BalanceQuota != 200 || wallet.TotalRechargeQuota != 200 {
		t.Fatalf("wallet = %+v, want recharge totals", wallet)
	}

	var record domains.UserWalletRecord
	if err := db.Where("payment_guid = ?", paid.Guid).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.OrderNo != order.OrderNo || record.AmountCents != 500 || record.QuotaDelta != 200 {
		t.Fatalf("wallet record = %+v, want payment income record", record)
	}
}

func TestConfirmWechatTransactionRejectsAmountMismatch(t *testing.T) {
	db := withWechatPaymentTestDB(t)
	order := createWechatPaymentOrder(t, 500, 200)
	transaction := &payments.Transaction{
		OutTradeNo:    core.String(order.OrderNo),
		TradeState:    core.String("SUCCESS"),
		TransactionId: core.String("420000000000002"),
		Amount: &payments.TransactionAmount{
			Total:    core.Int64(499),
			Currency: core.String("CNY"),
		},
	}

	_, err := PaymentServiceApp.ConfirmWechatTransaction(transaction, "")
	if err == nil || !strings.Contains(err.Error(), "amount") {
		t.Fatalf("err = %v, want amount mismatch", err)
	}

	var saved domains.PaymentOrder
	if err := db.Where("order_no = ?", order.OrderNo).First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != "pending" {
		t.Fatalf("status = %s, want pending", saved.Status)
	}
}

func TestSetWechatPaySettingsPreservesMaskedSecrets(t *testing.T) {
	withWechatPaymentTestDB(t)
	if err := OptionServiceApp.Set(optionWechatPayMchPrivateKey, "private-key"); err != nil {
		t.Fatal(err)
	}
	if err := OptionServiceApp.Set(optionWechatPayAPIv3Key, "api-v3-key"); err != nil {
		t.Fatal(err)
	}

	err := PaymentServiceApp.SetWechatPaySettings(WechatPaySettings{
		Enabled:                false,
		AppID:                  "wx-app",
		MchID:                  "mch-001",
		MchCertificateSerialNo: "serial-001",
		MchPrivateKey:          "******",
		APIv3Key:               "******",
		NotifyURL:              "https://example.com/payment/wechat/notify",
		DescriptionPrefix:      "NavAPI",
	})
	if err != nil {
		t.Fatal(err)
	}

	settings := PaymentServiceApp.GetWechatPaySettings()
	if settings.MchPrivateKey != "private-key" || settings.APIv3Key != "api-v3-key" {
		t.Fatalf("settings = %+v, want preserved secrets", settings)
	}
}

func createWechatPaymentOrder(t *testing.T, amountCents int64, quota int64) domains.PaymentOrder {
	t.Helper()
	order := domains.PaymentOrder{
		OrderNo:     newOrderNo(),
		UserGuid:    "user-wechat",
		Type:        "quota",
		Status:      "pending",
		Provider:    paymentProviderWechat,
		TradeType:   wechatTradeTypeNative,
		AmountCents: amountCents,
		Currency:    "CNY",
		Quota:       quota,
	}
	if err := createWithCrud(&PaymentServiceApp.CrudService, &order); err != nil {
		t.Fatal(err)
	}
	return order
}

func withWechatPaymentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	previousCache := OptionServiceApp.cache
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&domains.PaymentOrder{},
		&domains.UserQuota{},
		&domains.UserWallet{},
		&domains.UserWalletRecord{},
		&domains.Option{},
		&domains.Setting{},
	); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	OptionServiceApp.cache = map[string]string{}
	t.Cleanup(func() {
		global.NAV_DB = previousDB
		OptionServiceApp.cache = previousCache
	})
	return db
}
