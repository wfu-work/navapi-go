package domains

import (
	"os"

	"github.com/wfu-work/nav-common-go-lib/global"
	"go.uber.org/zap"
)

func RegisterTables() {
	db := global.NAV_DB
	if db == nil {
		return
	}
	if err := migrateLegacyProbeLogTable(db); err != nil {
		global.NAV_LOG.Error("migrate legacy probe log table failed", zap.Error(err))
		os.Exit(1)
	}
	if err := db.AutoMigrate(
		ApiToken{},
		UserWallet{},
		UserWalletRecord{},
		UserSettings{},
		UsageLog{},
		ProbeLog{},
		Announcement{},
		ModelMeta{},
		ModelGroup{},
		VendorMeta{},
		Pricing{},
		Option{},
		Task{},
		Redemption{},
		SubscriptionPlan{},
		UserSubscription{},
		PaymentOrder{},
		InvitationCode{},
		InvitationRelation{},
		CheckinRecord{},
		QuotaDate{},
		MessageEmailConfig{},
		MessageTemplate{},
		MessageSendRecord{},
		MessageEmailCode{},
		Setting{},
	); err != nil {
		global.NAV_LOG.Error("register navapi business tables failed", zap.Error(err))
		os.Exit(1)
	}
	global.NAV_LOG.Info("register navapi business tables success")
}
