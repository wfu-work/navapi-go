package routers

import (
	"navapi-go/apis"

	"github.com/gin-gonic/gin"
)

var RouterGroupApp = new(RouterGroup)

type RouterGroup struct {
	GatewayRouter
	TokenRouter
	LogRouter
	ModelRouter
	RelayRouter
	OptionRouter
	TaskRouter
	RedemptionRouter
	PricingRouter
	QuotaRouter
	ProviderRouter
	AnnouncementRouter
	SubscriptionRouter
	PaymentRouter
	WalletRouter
	InvitationRouter
	CheckinRouter
	ClientsRouter
	MessageRouter
	RegisterRouter
	UserSettingsRouter
	SettingRouter
}

var (
	gatewayApi            = apis.ApiGroupApp.GatewayApi
	tokenApi              = apis.ApiGroupApp.TokenApi
	logApi                = apis.ApiGroupApp.UsageLogApi
	modelApi              = apis.ApiGroupApp.ModelApi
	relayApi              = apis.ApiGroupApp.RelayApi
	optionApi             = apis.ApiGroupApp.OptionApi
	taskApi               = apis.ApiGroupApp.TaskApi
	redemptionApi         = apis.ApiGroupApp.RedemptionApi
	pricingApi            = apis.ApiGroupApp.PricingApi
	quotaApi              = apis.ApiGroupApp.QuotaApi
	providerApi           = apis.ApiGroupApp.ProviderApi
	announcementApi       = apis.ApiGroupApp.AnnouncementApi
	subscriptionApi       = apis.ApiGroupApp.SubscriptionApi
	paymentApi            = apis.ApiGroupApp.PaymentApi
	walletApi             = apis.ApiGroupApp.WalletApi
	invitationApi         = apis.ApiGroupApp.InvitationApi
	checkinApi            = apis.ApiGroupApp.CheckinApi
	messageEmailConfigApi = apis.ApiGroupApp.MessageEmailConfigApi
	messageTemplateApi    = apis.ApiGroupApp.MessageTemplateApi
	messageSendRecordApi  = apis.ApiGroupApp.MessageSendRecordApi
	registerApi           = apis.ApiGroupApp.RegisterApi
	userSettingsApi       = apis.ApiGroupApp.UserSettingsApi
	settingApi            = apis.ApiGroupApp.SettingApi
)

func (r *RouterGroup) InitRouters(publicGroup *gin.RouterGroup, privateGroup *gin.RouterGroup) {
	r.InitRelayRouter(publicGroup)
	r.InitGatewayRouter(privateGroup, publicGroup)
	r.InitTokenRouter(privateGroup)
	r.InitLogRouter(privateGroup)
	r.InitModelRouter(privateGroup, publicGroup)
	r.InitOptionRouter(privateGroup, publicGroup)
	r.InitTaskRouter(privateGroup)
	r.InitRedemptionRouter(privateGroup)
	r.InitPricingRouter(privateGroup, publicGroup)
	r.InitQuotaRouter(privateGroup)
	r.InitProviderRouter(privateGroup)
	r.InitAnnouncementRouter(privateGroup, publicGroup)
	r.InitSubscriptionRouter(privateGroup, publicGroup)
	r.InitPaymentRouter(privateGroup, publicGroup)
	r.InitWalletRouter(privateGroup, publicGroup)
	r.InitInvitationRouter(privateGroup)
	r.InitCheckinRouter(privateGroup)
	r.InitClientsRouter(privateGroup)
	r.InitMessageRouter(privateGroup)
	r.InitRegisterRouter(publicGroup)
	r.InitUserSettingsRouter(privateGroup)
	r.InitSettingRouter(privateGroup, publicGroup)
}
