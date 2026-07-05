package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type UserSettings struct {
	commonDomains.BaseDataEntity
	UserGuid                    string `json:"userGuid" gorm:"column:user_guid;size:100;uniqueIndex;comment:用户 GUID"`
	QuotaReminderEnabled        bool   `json:"quotaReminderEnabled" gorm:"column:quota_reminder_enabled;default:true;comment:额度提醒开关"`
	PlatformAnnouncementEnabled bool   `json:"platformAnnouncementEnabled" gorm:"column:platform_announcement_enabled;default:true;comment:平台公告开关"`
	AbnormalCallAlertEnabled    bool   `json:"abnormalCallAlertEnabled" gorm:"column:abnormal_call_alert_enabled;default:false;comment:异常调用提醒开关"`
	MaxConcurrency              int    `json:"maxConcurrency" gorm:"column:max_concurrency;default:5;comment:用户最大并发数"`
	ExtraConfig                 string `json:"extraConfig" gorm:"column:extra_config;type:text;comment:扩展 JSON 配置"`
}

func (UserSettings) TableName() string {
	return "nav_api_user_settings"
}
