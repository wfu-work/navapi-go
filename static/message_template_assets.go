package static

import _ "embed"

//go:embed message_templates/register_email_code.html
var RegisterEmailCodeTemplateHTML string

//go:embed message_templates/user_balance_insufficient.html
var UserBalanceInsufficientTemplateHTML string

//go:embed message_templates/user_daily_usage_bill.html
var UserDailyUsageBillTemplateHTML string

//go:embed message_templates/admin_daily_usage_bill.html
var AdminDailyUsageBillTemplateHTML string

//go:embed message_templates/platform_announcement.html
var PlatformAnnouncementTemplateHTML string
