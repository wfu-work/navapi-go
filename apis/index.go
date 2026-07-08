package apis

import (
	"navapi-go/services"

	"github.com/gin-gonic/gin"
)

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
	ClientUserApi
	UserSettingsApi
	SettingApi
	SystemMonitorApi
}

var (
	gatewayService            = services.ServiceGroupApp.GatewayService
	relayService              = services.ServiceGroupApp.RelayService
	riskControlService        = services.ServiceGroupApp.RiskControlService
	registerSettingService    = services.ServiceGroupApp.RegisterSettingService
	emailService              = services.ServiceGroupApp.EmailService
	clientRegisterService     = services.ServiceGroupApp.ClientRegisterService
	clientUserService         = services.ServiceGroupApp.ClientUserService
	tokenService              = services.ServiceGroupApp.TokenService
	logService                = services.ServiceGroupApp.LogService
	modelService              = services.ServiceGroupApp.ModelService
	providerService           = services.ServiceGroupApp.ProviderService
	pricingService            = services.ServiceGroupApp.PricingService
	optionService             = services.ServiceGroupApp.OptionService
	taskService               = services.ServiceGroupApp.TaskService
	redemptionService         = services.ServiceGroupApp.RedemptionService
	subscriptionService       = services.ServiceGroupApp.SubscriptionService
	paymentService            = services.ServiceGroupApp.PaymentService
	userWalletService         = services.ServiceGroupApp.UserWalletService
	userSettingsService       = services.ServiceGroupApp.UserSettingsService
	announcementService       = services.ServiceGroupApp.AnnouncementService
	invitationService         = services.ServiceGroupApp.InvitationService
	checkinService            = services.ServiceGroupApp.CheckinService
	settingService            = services.ServiceGroupApp.SettingService
	messageEmailConfigService = services.ServiceGroupApp.MessageEmailConfigService
	messageTemplateService    = services.ServiceGroupApp.MessageTemplateService
	messageSendRecordService  = services.ServiceGroupApp.MessageSendRecordService
	systemMonitorService      = services.ServiceGroupApp.SystemMonitorService
)

func queryParams(c *gin.Context) map[string]string {
	params := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return params
}
