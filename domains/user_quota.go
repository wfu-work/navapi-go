package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type UserQuota struct {
	commonDomains.BaseDataEntity
	UserGuid           string `json:"userGuid" gorm:"column:user_guid;size:100;uniqueIndex;comment:用户 GUID"`
	RemainAmountMicros int64  `json:"remainAmountMicros" gorm:"column:remain_amount_micros;default:0;comment:剩余金额微单位"`
	UsedAmountMicros   int64  `json:"usedAmountMicros" gorm:"column:used_amount_micros;default:0;comment:已用金额微单位"`
	TotalAmountMicros  int64  `json:"totalAmountMicros" gorm:"column:total_amount_micros;default:0;comment:累计入账金额微单位"`
	Group              string `json:"group" gorm:"column:group_name;size:100;default:default;comment:默认分组"`
	AllowedGroups      string `json:"allowedGroups" gorm:"column:allowed_groups;type:text;comment:逗号分隔可用分组，空表示不限制"`
}

func (UserQuota) TableName() string {
	return "nav_api_user_quotas"
}
