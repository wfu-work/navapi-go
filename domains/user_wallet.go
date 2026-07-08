package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

const (
	WalletRecordTypeRecharge     = "recharge"
	WalletRecordTypeSubscription = "subscription"
	WalletRecordTypeConsume      = "consume"
	WalletRecordTypeReward       = "reward"
	WalletRecordTypeCommission   = "commission"

	WalletRecordDirectionIncome  = "income"
	WalletRecordDirectionOutcome = "outcome"

	WalletSourcePayment      = "payment"
	WalletSourceSubscription = "subscription"
	WalletSourceRelay        = "relay"
	WalletSourceCheckin      = "checkin"
	WalletSourceInvitation   = "invitation"
	WalletSourceRedemption   = "redemption"
	WalletSourceManual       = "manual"
)

type UserWallet struct {
	commonDomains.BaseDataEntity
	UserGuid                      string `json:"userGuid" gorm:"column:user_guid;size:100;uniqueIndex;comment:用户 GUID"`
	BalanceAmountMicros           int64  `json:"balanceAmountMicros" gorm:"column:balance_amount_micros;default:0;comment:当前余额金额微单位"`
	PaidBalanceAmountMicros       int64  `json:"paidBalanceAmountMicros" gorm:"column:paid_balance_amount_micros;default:0;comment:付费余额金额微单位"`
	RewardBalanceAmountMicros     int64  `json:"rewardBalanceAmountMicros" gorm:"column:reward_balance_amount_micros;default:0;comment:奖励余额金额微单位"`
	CommissionBalanceAmountMicros int64  `json:"commissionBalanceAmountMicros" gorm:"column:commission_balance_amount_micros;default:0;comment:邀请分佣余额金额微单位"`
	TotalConsumedAmountMicros     int64  `json:"totalConsumedAmountMicros" gorm:"column:total_consumed_amount_micros;default:0;comment:累计消费金额微单位"`
	TotalRequestCount             int64  `json:"totalRequestCount" gorm:"column:total_request_count;default:0;comment:累计 API 请求数"`
	TotalRechargeAmountMicros     int64  `json:"totalRechargeAmountMicros" gorm:"column:total_recharge_amount_micros;default:0;comment:累计充值入账金额微单位"`
	TotalSubscriptionAmountMicros int64  `json:"totalSubscriptionAmountMicros" gorm:"column:total_subscription_amount_micros;default:0;comment:累计订阅入账金额微单位"`
	TotalRewardAmountMicros       int64  `json:"totalRewardAmountMicros" gorm:"column:total_reward_amount_micros;default:0;comment:累计奖励金额微单位"`
	TotalCommissionAmountMicros   int64  `json:"totalCommissionAmountMicros" gorm:"column:total_commission_amount_micros;default:0;comment:累计邀请分佣金额微单位"`
	TotalRechargeAmountCents      int64  `json:"totalRechargeAmountCents" gorm:"column:total_recharge_amount_cents;default:0;comment:累计充值金额分"`
	TotalSubscriptionAmountCents  int64  `json:"totalSubscriptionAmountCents" gorm:"column:total_subscription_amount_cents;default:0;comment:累计订阅金额分"`
	Currency                      string `json:"currency" gorm:"column:currency;size:20;default:CNY;comment:币种"`
}

func (UserWallet) TableName() string {
	return "nav_api_user_wallets"
}

type UserWalletRecord struct {
	commonDomains.BaseDataEntity
	UserGuid                           string `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	Type                               string `json:"type" gorm:"column:type;size:30;index;comment:流水类型 recharge/subscription/consume/reward/commission"`
	Direction                          string `json:"direction" gorm:"column:direction;size:20;index;comment:方向 income/outcome"`
	Source                             string `json:"source" gorm:"column:source;size:60;index;comment:来源 payment/subscription/relay/checkin/invitation/redemption/manual"`
	Title                              string `json:"title" gorm:"column:title;size:120;comment:标题"`
	RequestCountDelta                  int64  `json:"requestCountDelta" gorm:"column:request_count_delta;default:0;comment:请求数变动"`
	AmountMicrosDelta                  int64  `json:"amountMicrosDelta" gorm:"column:amount_micros_delta;default:0;comment:金额微单位变动，收入为正消费为负"`
	BalanceAmountMicrosAfter           int64  `json:"balanceAmountMicrosAfter" gorm:"column:balance_amount_micros_after;default:0;comment:变动后余额金额微单位"`
	PaidBalanceAmountMicrosAfter       int64  `json:"paidBalanceAmountMicrosAfter" gorm:"column:paid_balance_amount_micros_after;default:0;comment:变动后付费余额金额微单位"`
	RewardBalanceAmountMicrosAfter     int64  `json:"rewardBalanceAmountMicrosAfter" gorm:"column:reward_balance_amount_micros_after;default:0;comment:变动后奖励余额金额微单位"`
	CommissionBalanceAmountMicrosAfter int64  `json:"commissionBalanceAmountMicrosAfter" gorm:"column:commission_balance_amount_micros_after;default:0;comment:变动后邀请分佣余额金额微单位"`
	AmountCents                        int64  `json:"amountCents" gorm:"column:amount_cents;default:0;comment:金额分"`
	Currency                           string `json:"currency" gorm:"column:currency;size:20;default:CNY;comment:币种"`
	OrderNo                            string `json:"orderNo" gorm:"column:order_no;size:100;index;comment:支付订单号"`
	PaymentGuid                        string `json:"paymentGuid" gorm:"column:payment_guid;size:100;index;comment:支付订单 GUID"`
	SubscriptionGuid                   string `json:"subscriptionGuid" gorm:"column:subscription_guid;size:100;index;comment:订阅 GUID"`
	TokenID                            uint   `json:"tokenId" gorm:"column:token_id;index;comment:Token ID"`
	TokenGuid                          string `json:"tokenGuid" gorm:"column:token_guid;size:100;index;comment:Token GUID"`
	RelatedGuid                        string `json:"relatedGuid" gorm:"column:related_guid;size:100;index;comment:关联业务 GUID"`
	OccurredAt                         int64  `json:"occurredAt" gorm:"column:occurred_at;default:0;index;comment:发生时间秒"`
	Remark                             string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
	Meta                               string `json:"meta" gorm:"column:meta;type:text;comment:扩展 JSON"`
}

func (UserWalletRecord) TableName() string {
	return "nav_api_user_wallet_records"
}
