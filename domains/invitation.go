package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type InvitationCode struct {
	commonDomains.BaseDataEntity
	Code               string `json:"code" gorm:"column:code;size:80;uniqueIndex;comment:邀请码"`
	OwnerUserGuid      string `json:"ownerUserGuid" gorm:"column:owner_user_guid;size:100;index;comment:邀请人用户 GUID"`
	Status             int    `json:"status" gorm:"column:status;default:1;index;comment:状态"`
	MaxUses            int    `json:"maxUses" gorm:"column:max_uses;default:0;comment:最大使用次数，0 不限制"`
	UsedCount          int    `json:"usedCount" gorm:"column:used_count;default:0;comment:已使用次数"`
	RewardQuota        int64  `json:"rewardQuota" gorm:"column:reward_quota;default:0;comment:邀请人奖励额度"`
	InviteeRewardQuota int64  `json:"inviteeRewardQuota" gorm:"column:invitee_reward_quota;default:0;comment:被邀请人奖励额度"`
	ExpiredAt          int64  `json:"expiredAt" gorm:"column:expired_at;default:0;index;comment:过期时间秒"`
	Remark             string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (InvitationCode) TableName() string {
	return "nav_api_invitation_codes"
}

type InvitationRelation struct {
	commonDomains.BaseDataEntity
	Code               string `json:"code" gorm:"column:code;size:80;index;comment:邀请码"`
	InviterUserGuid    string `json:"inviterUserGuid" gorm:"column:inviter_user_guid;size:100;index;comment:邀请人用户 GUID"`
	InviteeUserGuid    string `json:"inviteeUserGuid" gorm:"column:invitee_user_guid;size:100;uniqueIndex;comment:被邀请人用户 GUID"`
	RewardQuota        int64  `json:"rewardQuota" gorm:"column:reward_quota;default:0;comment:邀请人奖励额度"`
	InviteeRewardQuota int64  `json:"inviteeRewardQuota" gorm:"column:invitee_reward_quota;default:0;comment:被邀请人奖励额度"`
	Rewarded           bool   `json:"rewarded" gorm:"column:rewarded;default:false;index;comment:是否已发奖励"`
	RewardedAt         int64  `json:"rewardedAt" gorm:"column:rewarded_at;default:0;comment:奖励时间秒"`
	Remark             string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (InvitationRelation) TableName() string {
	return "nav_api_invitation_relations"
}
