package routers

import (
	"navapi-go/apis"

	"github.com/gin-gonic/gin"
)

var RouterGroupApp = new(RouterGroup)

type RouterGroup struct {
	ChannelRouter
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
	InvitationRouter
	CheckinRouter
}

var (
	channelApi      = apis.ApiGroupApp.ChannelApi
	tokenApi        = apis.ApiGroupApp.TokenApi
	logApi          = apis.ApiGroupApp.UsageLogApi
	modelApi        = apis.ApiGroupApp.ModelApi
	relayApi        = apis.ApiGroupApp.RelayApi
	optionApi       = apis.ApiGroupApp.OptionApi
	taskApi         = apis.ApiGroupApp.TaskApi
	redemptionApi   = apis.ApiGroupApp.RedemptionApi
	pricingApi      = apis.ApiGroupApp.PricingApi
	quotaApi        = apis.ApiGroupApp.QuotaApi
	providerApi     = apis.ApiGroupApp.ProviderApi
	announcementApi = apis.ApiGroupApp.AnnouncementApi
	subscriptionApi = apis.ApiGroupApp.SubscriptionApi
	paymentApi      = apis.ApiGroupApp.PaymentApi
	invitationApi   = apis.ApiGroupApp.InvitationApi
	checkinApi      = apis.ApiGroupApp.CheckinApi
)

func (r *RouterGroup) InitRouters(publicGroup *gin.RouterGroup, privateGroup *gin.RouterGroup, engine *gin.Engine) {
	r.InitChannelRouter(privateGroup)
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
	r.InitPaymentRouter(privateGroup)
	r.InitInvitationRouter(privateGroup)
	r.InitCheckinRouter(privateGroup)
}
