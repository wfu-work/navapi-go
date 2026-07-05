package services

import "time"

const (
	MessageChannelEmail         = "email"
	MessageSendStatusPending    = "pending"
	MessageSendStatusSuccess    = "success"
	MessageSendStatusFailed     = "failed"
	MessageReceiveStatusWaiting = "waiting"
	MessageReceiveStatusDone    = "accepted"
	MessageReceiveStatusFailed  = "failed"
	MessageEmailCodePending     = "pending"
	MessageEmailCodeUsed        = "used"
	MessageEmailCodeExpired     = "expired"
	MessageSceneRegister        = "register"
	TemplateCodeRegisterCaptcha = "register_email_code"
	TemplateCodeEmailConfigTest = "email_config_test"
	MaxMessageSendRetries       = 3
	RegisterEmailCodeTTL        = 10 * time.Minute
)

func nowMilli() int64 {
	return time.Now().UnixMilli()
}
