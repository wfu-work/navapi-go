package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type SubscriptionPlan struct {
	commonDomains.BaseDataEntity
	Name          string `json:"name" gorm:"column:name;size:100;index;comment:套餐名称"`
	Code          string `json:"code" gorm:"column:code;size:80;uniqueIndex;comment:套餐编码"`
	Status        int    `json:"status" gorm:"column:status;default:1;index;comment:状态"`
	WeeklyAmount  int64  `json:"weeklyAmount" gorm:"column:weekly_amount;default:0;comment:周限金额"`
	Amount        int64  `json:"amount" gorm:"column:amount;default:0;comment:订阅入账金额"`
	DurationDays  int    `json:"durationDays" gorm:"column:duration_days;default:30;comment:有效天数"`
	PriceCents    int64  `json:"priceCents" gorm:"column:price_cents;default:0;comment:价格分"`
	Currency      string `json:"currency" gorm:"column:currency;size:20;default:CNY;comment:币种"`
	Group         string `json:"group" gorm:"column:group_name;size:100;default:default;comment:默认分组"`
	AllowedGroups string `json:"allowedGroups" gorm:"column:allowed_groups;type:text;comment:可用分组"`
	Sort          int    `json:"sort" gorm:"column:sort;default:0;index;comment:排序"`
	Remark        string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (SubscriptionPlan) TableName() string {
	return "nav_api_subscription_plans"
}

type UserSubscription struct {
	commonDomains.BaseDataEntity
	UserGuid     string `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	PlanGuid     string `json:"planGuid" gorm:"column:plan_guid;size:100;index;comment:套餐 GUID"`
	PlanCode     string `json:"planCode" gorm:"column:plan_code;size:80;index;comment:套餐编码"`
	PlanName     string `json:"planName" gorm:"column:plan_name;size:100;comment:套餐名称"`
	Status       string `json:"status" gorm:"column:status;size:30;default:active;index;comment:状态"`
	WeeklyAmount int64  `json:"weeklyAmount" gorm:"column:weekly_amount;default:0;comment:周限金额"`
	Amount       int64  `json:"amount" gorm:"column:amount;default:0;comment:本次入账金额"`
	StartAt      int64  `json:"startAt" gorm:"column:start_at;index;comment:开始时间秒"`
	EndAt        int64  `json:"endAt" gorm:"column:end_at;index;comment:结束时间秒"`
	PaymentGuid  string `json:"paymentGuid" gorm:"column:payment_guid;size:100;index;comment:支付订单 GUID"`
	RenewalCount int    `json:"renewalCount" gorm:"column:renewal_count;default:0;comment:续费次数"`
	Remark       string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (UserSubscription) TableName() string {
	return "nav_api_user_subscriptions"
}
