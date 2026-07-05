package services

import "time"

const (
	MessageChannelEmail                 = "email"
	MessageSendStatusPending            = "pending"
	MessageSendStatusSuccess            = "success"
	MessageSendStatusFailed             = "failed"
	MessageReceiveStatusWaiting         = "waiting"
	MessageReceiveStatusDone            = "accepted"
	MessageReceiveStatusFailed          = "failed"
	MessageEmailCodePending             = "pending"
	MessageEmailCodeUsed                = "used"
	MessageEmailCodeExpired             = "expired"
	MessageSceneRegister                = "register"
	TemplateCodeRegisterCaptcha         = "register_email_code"
	TemplateCodeUserBalanceInsufficient = "user_balance_insufficient"
	TemplateCodeUserDailyUsageBill      = "user_daily_usage_bill"
	TemplateCodeAdminDailyUsageBill     = "admin_daily_usage_bill"
	TemplateCodeEmailConfigTest         = "email_config_test"
	MaxMessageSendRetries               = 3
	RegisterEmailCodeTTL                = 10 * time.Minute
	RegisterEmailCodeCooldown           = 60 * time.Second
	RegisterEmailCodeHourlyLimit        = 10
)

func nowMilli() int64 {
	return time.Now().UnixMilli()
}
