package apis

import "github.com/gin-gonic/gin"

var ApiGroupApp = new(ApiGroup)

type ApiGroup struct {
	GatewayApi
	TokenApi
	UsageLogApi
	ModelApi
	RelayApi
	OptionApi
	TaskApi
	RedemptionApi
	PricingApi
	QuotaApi
	ProviderApi
	AnnouncementApi
	SubscriptionApi
	PaymentApi
	WalletApi
	InvitationApi
	CheckinApi
	MessageEmailConfigApi
	MessageTemplateApi
	MessageSendRecordApi
	RegisterApi
	UserSettingsApi
	SettingApi
}

func queryParams(c *gin.Context) map[string]string {
	params := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return params
}
