package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type Redemption struct {
	commonDomains.BaseDataEntity
	Code      string `json:"code" gorm:"column:code;size:80;uniqueIndex;comment:兑换码"`
	Amount    int64  `json:"amount" gorm:"column:amount;default:0;comment:兑换金额"`
	Status    int    `json:"status" gorm:"column:status;default:1;index;comment:状态"`
	UsedBy    string `json:"usedBy" gorm:"column:used_by;size:100;index;comment:使用用户 GUID"`
	Username  string `json:"username,omitempty" gorm:"-"`
	NickName  string `json:"nickName,omitempty" gorm:"-"`
	Email     string `json:"email,omitempty" gorm:"-"`
	UsedAt    int64  `json:"usedAt" gorm:"column:used_at;default:0;comment:使用时间"`
	ExpiredAt int64  `json:"expiredAt" gorm:"column:expired_at;default:0;index;comment:过期时间"`
	Remark    string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (Redemption) TableName() string {
	return "nav_api_redemptions"
}
