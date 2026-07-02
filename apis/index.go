package apis

var ApiGroupApp = new(ApiGroup)

type ApiGroup struct {
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
	InvitationApi
	CheckinApi
}
