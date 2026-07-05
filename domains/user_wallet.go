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
	WalletSourceManual       = "manual"
)

type UserWallet struct {
	commonDomains.BaseDataEntity
	UserGuid                     string `json:"userGuid" gorm:"column:user_guid;size:100;uniqueIndex;comment:用户 GUID"`
	BalanceQuota                 int64  `json:"balanceQuota" gorm:"column:balance_quota;default:0;comment:当前余额额度"`
	PaidBalanceQuota             int64  `json:"paidBalanceQuota" gorm:"column:paid_balance_quota;default:0;comment:付费余额额度"`
	RewardBalanceQuota           int64  `json:"rewardBalanceQuota" gorm:"column:reward_balance_quota;default:0;comment:奖励余额额度"`
	CommissionBalanceQuota       int64  `json:"commissionBalanceQuota" gorm:"column:commission_balance_quota;default:0;comment:邀请分佣余额额度"`
	TotalConsumedQuota           int64  `json:"totalConsumedQuota" gorm:"column:total_consumed_quota;default:0;comment:累计消费额度"`
	TotalRequestCount            int64  `json:"totalRequestCount" gorm:"column:total_request_count;default:0;comment:累计 API 请求数"`
	TotalRechargeQuota           int64  `json:"totalRechargeQuota" gorm:"column:total_recharge_quota;default:0;comment:累计充值额度"`
	TotalSubscriptionQuota       int64  `json:"totalSubscriptionQuota" gorm:"column:total_subscription_quota;default:0;comment:累计订阅额度"`
	TotalRewardQuota             int64  `json:"totalRewardQuota" gorm:"column:total_reward_quota;default:0;comment:累计奖励额度"`
	TotalCommissionQuota         int64  `json:"totalCommissionQuota" gorm:"column:total_commission_quota;default:0;comment:累计邀请分佣额度"`
	TotalRechargeAmountCents     int64  `json:"totalRechargeAmountCents" gorm:"column:total_recharge_amount_cents;default:0;comment:累计充值金额分"`
	TotalSubscriptionAmountCents int64  `json:"totalSubscriptionAmountCents" gorm:"column:total_subscription_amount_cents;default:0;comment:累计订阅金额分"`
	Currency                     string `json:"currency" gorm:"column:currency;size:20;default:CNY;comment:币种"`
}

func (UserWallet) TableName() string {
	return "nav_api_user_wallets"
}

type UserWalletRecord struct {
	commonDomains.BaseDataEntity
	UserGuid               string `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	Type                   string `json:"type" gorm:"column:type;size:30;index;comment:流水类型 recharge/subscription/consume/reward/commission"`
	Direction              string `json:"direction" gorm:"column:direction;size:20;index;comment:方向 income/outcome"`
	Source                 string `json:"source" gorm:"column:source;size:60;index;comment:来源 payment/subscription/relay/checkin/invitation/manual"`
	Title                  string `json:"title" gorm:"column:title;size:120;comment:标题"`
	QuotaDelta             int64  `json:"quotaDelta" gorm:"column:quota_delta;default:0;comment:额度变动，收入为正消费为负"`
	RequestCountDelta      int64  `json:"requestCountDelta" gorm:"column:request_count_delta;default:0;comment:请求数变动"`
	BalanceAfter           int64  `json:"balanceAfter" gorm:"column:balance_after;default:0;comment:变动后当前余额"`
	PaidBalanceAfter       int64  `json:"paidBalanceAfter" gorm:"column:paid_balance_after;default:0;comment:变动后付费余额"`
	RewardBalanceAfter     int64  `json:"rewardBalanceAfter" gorm:"column:reward_balance_after;default:0;comment:变动后奖励余额"`
	CommissionBalanceAfter int64  `json:"commissionBalanceAfter" gorm:"column:commission_balance_after;default:0;comment:变动后邀请分佣余额"`
	AmountCents            int64  `json:"amountCents" gorm:"column:amount_cents;default:0;comment:金额分"`
	Currency               string `json:"currency" gorm:"column:currency;size:20;default:CNY;comment:币种"`
	OrderNo                string `json:"orderNo" gorm:"column:order_no;size:100;index;comment:支付订单号"`
	PaymentGuid            string `json:"paymentGuid" gorm:"column:payment_guid;size:100;index;comment:支付订单 GUID"`
	SubscriptionGuid       string `json:"subscriptionGuid" gorm:"column:subscription_guid;size:100;index;comment:订阅 GUID"`
	TokenID                uint   `json:"tokenId" gorm:"column:token_id;index;comment:Token ID"`
	TokenGuid              string `json:"tokenGuid" gorm:"column:token_guid;size:100;index;comment:Token GUID"`
	RelatedGuid            string `json:"relatedGuid" gorm:"column:related_guid;size:100;index;comment:关联业务 GUID"`
	OccurredAt             int64  `json:"occurredAt" gorm:"column:occurred_at;default:0;index;comment:发生时间秒"`
	Remark                 string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
	Meta                   string `json:"meta" gorm:"column:meta;type:text;comment:扩展 JSON"`
}

func (UserWalletRecord) TableName() string {
	return "nav_api_user_wallet_records"
}
