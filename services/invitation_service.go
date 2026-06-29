package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvitationService struct{}

var InvitationServiceApp = InvitationService{}

type InviteSettings struct {
	Enabled            bool  `json:"enabled"`
	RequireInvite      bool  `json:"requireInvite"`
	RewardQuota        int64 `json:"rewardQuota"`
	InviteeRewardQuota int64 `json:"inviteeRewardQuota"`
	DefaultMaxUses     int   `json:"defaultMaxUses"`
	DefaultExpireDays  int   `json:"defaultExpireDays"`
}

type AcceptInviteRequest struct {
	Code    string `json:"code" binding:"required"`
	TokenID uint   `json:"tokenId"`
}

func (s InvitationService) Settings() InviteSettings {
	return InviteSettings{
		Enabled:            OptionServiceApp.Int64("invite.enabled", 1) > 0,
		RequireInvite:      OptionServiceApp.Int64("register.require_invite", 0) > 0,
		RewardQuota:        OptionServiceApp.Int64("invite.reward_quota", 0),
		InviteeRewardQuota: OptionServiceApp.Int64("invite.invitee_reward_quota", 0),
		DefaultMaxUses:     int(OptionServiceApp.Int64("invite.default_max_uses", 0)),
		DefaultExpireDays:  int(OptionServiceApp.Int64("invite.default_expire_days", 0)),
	}
}

func (s InvitationService) SetSettings(settings InviteSettings) error {
	values := map[string]string{
		"invite.reward_quota":         fmt.Sprint(settings.RewardQuota),
		"invite.invitee_reward_quota": fmt.Sprint(settings.InviteeRewardQuota),
		"invite.default_max_uses":     fmt.Sprint(settings.DefaultMaxUses),
		"invite.default_expire_days":  fmt.Sprint(settings.DefaultExpireDays),
	}
	if settings.Enabled {
		values["invite.enabled"] = "1"
	} else {
		values["invite.enabled"] = "0"
	}
	if settings.RequireInvite {
		values["register.require_invite"] = "1"
	} else {
		values["register.require_invite"] = "0"
	}
	for key, value := range values {
		if err := OptionServiceApp.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s InvitationService) ListCodes(ownerUserGuid string, query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var codes []domains.InvitationCode
	var total int64
	db := global.NAV_DB.Model(&domains.InvitationCode{})
	if ownerUserGuid != "" {
		db = db.Where("owner_user_guid = ?", ownerUserGuid)
	}
	if query.Q != "" {
		db = db.Where("code LIKE ? OR owner_user_guid LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&codes).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: codes, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s InvitationService) ListRelations(userGuid string, query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var relations []domains.InvitationRelation
	var total int64
	db := global.NAV_DB.Model(&domains.InvitationRelation{})
	if userGuid != "" {
		db = db.Where("inviter_user_guid = ? OR invitee_user_guid = ?", userGuid, userGuid)
	}
	if query.Q != "" {
		db = db.Where("code LIKE ? OR inviter_user_guid LIKE ? OR invitee_user_guid LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&relations).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: relations, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s InvitationService) SaveCode(code *domains.InvitationCode) error {
	if strings.TrimSpace(code.OwnerUserGuid) == "" {
		return errors.New("owner user guid is required")
	}
	if strings.TrimSpace(code.Code) == "" {
		generated, err := s.newCode()
		if err != nil {
			return err
		}
		code.Code = generated
	}
	if code.Status == 0 {
		code.Status = constants.StatusEnabled
	}
	settings := s.Settings()
	if code.MaxUses == 0 {
		code.MaxUses = settings.DefaultMaxUses
	}
	if code.RewardQuota == 0 {
		code.RewardQuota = settings.RewardQuota
	}
	if code.InviteeRewardQuota == 0 {
		code.InviteeRewardQuota = settings.InviteeRewardQuota
	}
	if code.ExpiredAt == 0 && settings.DefaultExpireDays > 0 {
		code.ExpiredAt = time.Now().AddDate(0, 0, settings.DefaultExpireDays).Unix()
	}
	return global.NAV_DB.Save(code).Error
}

func (s InvitationService) EnsureSelfCode(userGuid string) (*domains.InvitationCode, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	var code domains.InvitationCode
	err := global.NAV_DB.Where("owner_user_guid = ? AND status = ?", userGuid, constants.StatusEnabled).
		Order("id asc").
		First(&code).Error
	if err == nil {
		return &code, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	code = domains.InvitationCode{OwnerUserGuid: userGuid}
	if err := s.SaveCode(&code); err != nil {
		return nil, err
	}
	return &code, nil
}

func (s InvitationService) DeleteCode(id uint) error {
	return global.NAV_DB.Delete(&domains.InvitationCode{}, id).Error
}

// AcceptInvite binds the current user to an inviter exactly once and grants both
// sides their configured quota rewards in the same transaction.
func (s InvitationService) AcceptInvite(userGuid string, req AcceptInviteRequest) (*domains.InvitationRelation, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	settings := s.Settings()
	if !settings.Enabled {
		return nil, errors.New("invitation is disabled")
	}
	codeValue := strings.TrimSpace(req.Code)
	if codeValue == "" {
		return nil, errors.New("invite code is required")
	}
	var relation domains.InvitationRelation
	err := global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		var existing domains.InvitationRelation
		err := tx.Where("invitee_user_guid = ?", userGuid).First(&existing).Error
		if err == nil {
			return errors.New("invite has already been accepted")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var code domains.InvitationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", codeValue).First(&code).Error; err != nil {
			return err
		}
		if code.Status != constants.StatusEnabled {
			return errors.New("invite code is disabled")
		}
		if code.ExpiredAt > 0 && code.ExpiredAt < time.Now().Unix() {
			return errors.New("invite code is expired")
		}
		if code.MaxUses > 0 && code.UsedCount >= code.MaxUses {
			return errors.New("invite code usage limit reached")
		}
		if code.OwnerUserGuid == userGuid {
			return errors.New("cannot accept your own invite code")
		}
		relation = domains.InvitationRelation{
			Code:               code.Code,
			InviterUserGuid:    code.OwnerUserGuid,
			InviteeUserGuid:    userGuid,
			RewardQuota:        code.RewardQuota,
			InviteeRewardQuota: code.InviteeRewardQuota,
			Rewarded:           true,
			RewardedAt:         time.Now().Unix(),
		}
		if err := tx.Create(&relation).Error; err != nil {
			return err
		}
		if code.OwnerUserGuid != "" && code.RewardQuota > 0 {
			if err := UserQuotaServiceApp.Recharge(tx, code.OwnerUserGuid, 0, code.RewardQuota); err != nil {
				return err
			}
		}
		if code.InviteeRewardQuota > 0 {
			if err := UserQuotaServiceApp.Recharge(tx, userGuid, req.TokenID, code.InviteeRewardQuota); err != nil {
				return err
			}
		}
		code.UsedCount++
		return tx.Model(&domains.InvitationCode{}).Where("id = ?", code.Id).Update("used_count", code.UsedCount).Error
	})
	if err != nil {
		return nil, err
	}
	return &relation, nil
}

func (s InvitationService) Stats(userGuid string) (map[string]any, error) {
	result := map[string]any{}
	codeDB := global.NAV_DB.Model(&domains.InvitationCode{})
	relDB := global.NAV_DB.Model(&domains.InvitationRelation{})
	if userGuid != "" {
		codeDB = codeDB.Where("owner_user_guid = ?", userGuid)
		relDB = relDB.Where("inviter_user_guid = ?", userGuid)
	}
	var totalCodes, totalRelations int64
	if err := codeDB.Count(&totalCodes).Error; err != nil {
		return nil, err
	}
	if err := relDB.Count(&totalRelations).Error; err != nil {
		return nil, err
	}
	var sums struct {
		RewardQuota        int64
		InviteeRewardQuota int64
	}
	if err := relDB.Select("COALESCE(SUM(reward_quota),0) AS reward_quota, COALESCE(SUM(invitee_reward_quota),0) AS invitee_reward_quota").Scan(&sums).Error; err != nil {
		return nil, err
	}
	result["totalCodes"] = totalCodes
	result["totalInvites"] = totalRelations
	result["rewardQuota"] = sums.RewardQuota
	result["inviteeRewardQuota"] = sums.InviteeRewardQuota
	return result, nil
}

func (s InvitationService) newCode() (string, error) {
	raw, err := randomHex(6)
	if err != nil {
		return "", err
	}
	return "INV-" + strings.ToUpper(raw), nil
}
