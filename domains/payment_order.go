package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type PaymentOrder struct {
	commonDomains.BaseDataEntity
	OrderNo       string `json:"orderNo" gorm:"column:order_no;size:100;uniqueIndex;comment:订单号"`
	UserGuid      string `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	TokenID       uint   `json:"tokenId" gorm:"column:token_id;index;comment:充值目标 Token ID"`
	TokenGuid     string `json:"tokenGuid" gorm:"column:token_guid;size:100;index;comment:充值目标 Token GUID"`
	Type          string `json:"type" gorm:"column:type;size:30;index;comment:订单类型 recharge/subscription"`
	Status        string `json:"status" gorm:"column:status;size:30;default:pending;index;comment:状态"`
	Provider      string `json:"provider" gorm:"column:provider;size:40;index;comment:支付提供方"`
	AmountCents   int64  `json:"amountCents" gorm:"column:amount_cents;default:0;comment:金额分"`
	AmountMicros  int64  `json:"amountMicros" gorm:"column:amount_micros;default:0;comment:入账金额微单位"`
	Currency      string `json:"currency" gorm:"column:currency;size:20;default:CNY;comment:币种"`
	PlanGuid      string `json:"planGuid" gorm:"column:plan_guid;size:100;index;comment:订阅套餐 GUID"`
	PlanCode      string `json:"planCode" gorm:"column:plan_code;size:80;index;comment:订阅套餐编码"`
	TradeType     string `json:"tradeType" gorm:"column:trade_type;size:40;comment:三方交易类型"`
	CodeURL       string `json:"codeUrl" gorm:"column:code_url;type:text;comment:微信支付二维码链接"`
	PrepayID      string `json:"prepayId" gorm:"column:prepay_id;size:120;comment:预支付会话标识"`
	TransactionID string `json:"transactionId" gorm:"column:transaction_id;size:120;index;comment:三方交易号"`
	PaidAt        int64  `json:"paidAt" gorm:"column:paid_at;default:0;index;comment:支付时间秒"`
	ClosedAt      int64  `json:"closedAt" gorm:"column:closed_at;default:0;comment:关闭时间秒"`
	NotifyData    string `json:"notifyData" gorm:"column:notify_data;type:text;comment:通知原始数据"`
	Remark        string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (PaymentOrder) TableName() string {
	return "nav_api_payment_orders"
}
