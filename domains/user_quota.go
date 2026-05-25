package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type UserQuota struct {
	commonDomains.BaseDataEntity
	UserGuid      string `json:"userGuid" gorm:"column:user_guid;size:100;uniqueIndex;comment:用户 GUID"`
	RemainQuota   int64  `json:"remainQuota" gorm:"column:remain_quota;default:0;comment:剩余额度"`
	UsedQuota     int64  `json:"usedQuota" gorm:"column:used_quota;default:0;comment:已用额度"`
	TotalQuota    int64  `json:"totalQuota" gorm:"column:total_quota;default:0;comment:累计充值额度"`
	Group         string `json:"group" gorm:"column:group_name;size:100;default:default;comment:默认分组"`
	AllowedGroups string `json:"allowedGroups" gorm:"column:allowed_groups;type:text;comment:逗号分隔可用分组，空表示不限制"`
}

func (UserQuota) TableName() string {
	return "nav_api_user_quotas"
}
