package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type CheckinRecord struct {
	commonDomains.BaseDataEntity
	UserGuid    string `json:"userGuid" gorm:"column:user_guid;size:100;index:idx_checkin_user_date,unique;comment:用户 GUID"`
	Date        string `json:"date" gorm:"column:date;size:20;index:idx_checkin_user_date,unique;comment:日期 yyyy-mm-dd"`
	RewardQuota int64  `json:"rewardQuota" gorm:"column:reward_quota;default:0;comment:奖励额度"`
	Streak      int    `json:"streak" gorm:"column:streak;default:1;comment:连续签到天数"`
	TokenID     uint   `json:"tokenId" gorm:"column:token_id;index;comment:奖励目标 Token ID"`
	Status      string `json:"status" gorm:"column:status;size:30;default:success;index;comment:状态"`
	Remark      string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (CheckinRecord) TableName() string {
	return "nav_api_checkin_records"
}
