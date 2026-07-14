package services

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

var ServiceGroupApp = &ServiceGroup{
	GatewayService:            GatewayServiceApp,
	RelayService:              RelayServiceApp,
	RiskControlService:        RiskControlServiceApp,
	RegisterSettingService:    RegisterSettingServiceApp,
	EmailService:              EmailServiceApp,
	ClientRegisterService:     ClientRegisterServiceApp,
	ClientUserService:         ClientUserServiceApp,
	SystemMonitorService:      SystemMonitorServiceApp,
	TokenService:              TokenServiceApp,
	LogService:                LogServiceApp,
	ModelService:              ModelServiceApp,
	ProviderService:           ProviderServiceApp,
	PricingService:            PricingServiceApp,
	OptionService:             OptionServiceApp,
	TaskService:               TaskServiceApp,
	RedemptionService:         RedemptionServiceApp,
	SubscriptionService:       SubscriptionServiceApp,
	PaymentService:            PaymentServiceApp,
	UserWalletService:         UserWalletServiceApp,
	UserSettingsService:       UserSettingsServiceApp,
	UserConcurrencyService:    UserConcurrencyServiceApp,
	AnnouncementService:       AnnouncementServiceApp,
	InvitationService:         InvitationServiceApp,
	CheckinService:            CheckinServiceApp,
	RateLimitService:          RateLimitServiceApp,
	PermissionSeedService:     PermissionSeedServiceApp,
	SettingService:            SettingServiceApp,
	MessageEmailConfigService: MessageEmailConfigServiceApp,
	MessageEmailCodeService:   MessageEmailCodeServiceApp,
	MessageSendRecordService:  MessageSendRecordServiceApp,
	MessageTemplateService:    MessageTemplateServiceApp,
}

type ServiceGroup struct {
	*GatewayService
	*RelayService
	*RiskControlService
	*RegisterSettingService
	*EmailService
	*ClientRegisterService
	*ClientUserService
	*SystemMonitorService
	*TokenService
	*LogService
	*ModelService
	*ProviderService
	*PricingService
	*OptionService
	*TaskService
	*RedemptionService
	*SubscriptionService
	*PaymentService
	*UserWalletService
	*UserSettingsService
	*UserConcurrencyService
	*AnnouncementService
	*InvitationService
	*CheckinService
	*RateLimitService
	*PermissionSeedService
	*SettingService
	*MessageEmailConfigService
	*MessageEmailCodeService
	*MessageSendRecordService
	*MessageTemplateService
}

type HasBaseData interface {
	GetBaseData() commonDomains.BaseDataEntity
}
