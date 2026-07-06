package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type ApiToken struct {
	commonDomains.BaseDataEntity
	UserGuid           string `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	Username           string `json:"username" gorm:"-"`
	Email              string `json:"email" gorm:"-"`
	Name               string `json:"name" gorm:"column:name;size:100;index;comment:令牌名称"`
	Key                string `json:"-" gorm:"column:key;size:128;uniqueIndex;comment:令牌 key"`
	MaskedKey          string `json:"key" gorm:"-"`
	Status             int    `json:"status" gorm:"column:status;default:1;index;comment:状态"`
	Group              string `json:"group" gorm:"column:group_name;size:100;default:default;index;comment:分组"`
	RemainQuota        int64  `json:"remainQuota" gorm:"column:remain_quota;default:0;comment:剩余额度"`
	UnlimitedQuota     bool   `json:"unlimitedQuota" gorm:"column:unlimited_quota;default:false;comment:不限额度"`
	UsedQuota          int64  `json:"usedQuota" gorm:"column:used_quota;default:0;comment:已用额度"`
	ExpiredTime        int64  `json:"expiredTime" gorm:"column:expired_time;default:-1;index;comment:过期时间秒"`
	AccessedTime       int64  `json:"accessedTime" gorm:"column:accessed_time;default:0;comment:最后访问时间秒"`
	ModelLimitsEnabled bool   `json:"modelLimitsEnabled" gorm:"column:model_limits_enabled;default:false;comment:是否限制模型"`
	ModelLimits        string `json:"modelLimits" gorm:"column:model_limits;type:text;comment:逗号分隔模型白名单"`
	AllowIPs           string `json:"allowIps" gorm:"column:allow_ips;type:text;comment:逗号或换行分隔 IP 白名单"`
}

func (ApiToken) TableName() string {
	return "nav_api_tokens"
}
