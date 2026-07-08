package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvitationService struct {
	CodeCrud     commonServices.CrudService[domains.InvitationCode]
	RelationCrud commonServices.CrudService[domains.InvitationRelation]
}

var InvitationServiceApp = new(InvitationService)

func (s *InvitationService) WithDB(db *gorm.DB) *InvitationService {
	cloned := *s
	cloned.CodeCrud = *s.CodeCrud.WithDB(db)
	cloned.RelationCrud = *s.RelationCrud.WithDB(db)
	return &cloned
}

type InviteSettings struct {
	Enabled             bool  `json:"enabled"`
	RequireInvite       bool  `json:"requireInvite"`
	RewardAmount        int64 `json:"rewardAmount"`
	InviteeRewardAmount int64 `json:"inviteeRewardAmount"`
	DefaultMaxUses      int   `json:"defaultMaxUses"`
	DefaultExpireDays   int   `json:"defaultExpireDays"`
}

type AcceptInviteRequest struct {
	Code    string `json:"code" binding:"required"`
	TokenID uint   `json:"tokenId"`
}

func (s *InvitationService) Settings() InviteSettings {
	return InviteSettings{
		Enabled:             OptionServiceApp.Int64("invite.enabled", 1) > 0,
		RequireInvite:       OptionServiceApp.Int64("register.require_invite", 0) > 0,
		RewardAmount:        OptionServiceApp.Int64("invite.reward_amount", 0),
		InviteeRewardAmount: OptionServiceApp.Int64("invite.invitee_reward_amount", 0),
		DefaultMaxUses:      int(OptionServiceApp.Int64("invite.default_max_uses", 0)),
		DefaultExpireDays:   int(OptionServiceApp.Int64("invite.default_expire_days", 0)),
	}
}

func (s *InvitationService) SetSettings(settings InviteSettings) error {
	values := map[string]string{
		"invite.reward_amount":         fmt.Sprint(settings.RewardAmount),
		"invite.invitee_reward_amount": fmt.Sprint(settings.InviteeRewardAmount),
		"invite.default_max_uses":      fmt.Sprint(settings.DefaultMaxUses),
		"invite.default_expire_days":   fmt.Sprint(settings.DefaultExpireDays),
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

func (s *InvitationService) ListCodes(ownerUserGuid string, query vos.PageQuery) (vos.PageResult, error) {
	query.Normalize()
	var codes []domains.InvitationCode
	var total int64
	db := s.CodeCrud.DB().Model(&domains.InvitationCode{})
	if ownerUserGuid != "" {
		db = db.Where("owner_user_guid = ?", ownerUserGuid)
	}
	if query.Q != "" {
		db = db.Where("code LIKE ? OR owner_user_guid LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&codes).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: codes, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *InvitationService) ListRelations(userGuid string, query vos.PageQuery) (vos.PageResult, error) {
	query.Normalize()
	var relations []domains.InvitationRelation
	var total int64
	db := s.RelationCrud.DB().Model(&domains.InvitationRelation{})
	if userGuid != "" {
		db = db.Where("inviter_user_guid = ? OR invitee_user_guid = ?", userGuid, userGuid)
	}
	if query.Q != "" {
		db = db.Where("code LIKE ? OR inviter_user_guid LIKE ? OR invitee_user_guid LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&relations).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: relations, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *InvitationService) SaveCode(code *domains.InvitationCode) error {
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
	if code.RewardAmount == 0 {
		code.RewardAmount = settings.RewardAmount
	}
	if code.InviteeRewardAmount == 0 {
		code.InviteeRewardAmount = settings.InviteeRewardAmount
	}
	if code.ExpiredAt == 0 && settings.DefaultExpireDays > 0 {
		code.ExpiredAt = time.Now().AddDate(0, 0, settings.DefaultExpireDays).Unix()
	}
	if code.Id == 0 {
		return createWithCrud(&s.CodeCrud, code)
	}
	existing, err := s.GetCode(code.Id)
	if err != nil {
		return err
	}
	code.Guid = existing.Guid
	code.CreateTime = existing.CreateTime
	code.Creater = existing.Creater
	updating := *code
	updating.Id = 0
	if err := createWithCrud(&s.CodeCrud, &updating); err != nil {
		return err
	}
	*code = updating
	return nil
}

func (s *InvitationService) GetCode(id uint) (*domains.InvitationCode, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	code, err := s.CodeCrud.GetById(id)
	if err != nil {
		return nil, err
	}
	if code == nil {
		return nil, errors.New("invitation code not found")
	}
	return code, nil
}

func (s *InvitationService) EnsureSelfCode(userGuid string) (*domains.InvitationCode, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	var code domains.InvitationCode
	err := s.CodeCrud.DB().Where("owner_user_guid = ? AND status = ?", userGuid, constants.StatusEnabled).
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

func (s *InvitationService) DeleteCode(id uint) error {
	return deleteByIDWithCrud(&s.CodeCrud, id, "invitation code not found")
}

// AcceptInvite binds the current user to an inviter exactly once and grants both
// sides their configured quota rewards in the same transaction.
func (s *InvitationService) AcceptInvite(userGuid string, req AcceptInviteRequest) (*domains.InvitationRelation, error) {
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
	err := s.CodeCrud.DB().Transaction(func(tx *gorm.DB) error {
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
			Code:                code.Code,
			InviterUserGuid:     code.OwnerUserGuid,
			InviteeUserGuid:     userGuid,
			RewardAmount:        code.RewardAmount,
			InviteeRewardAmount: code.InviteeRewardAmount,
			Rewarded:            true,
			RewardedAt:          time.Now().Unix(),
		}
		if err := relation.BeforeCreate(nil); err != nil {
			return err
		}
		relationCrud := s.RelationCrud.WithDB(tx)
		if err := relationCrud.Create(relation); err != nil {
			return err
		}
		if code.OwnerUserGuid != "" && code.RewardAmount > 0 {
			amountMicros := WholeAmountToMicros(code.RewardAmount)
			if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
				UserGuid:     code.OwnerUserGuid,
				Type:         domains.WalletRecordTypeCommission,
				Source:       domains.WalletSourceInvitation,
				Title:        "邀请分佣",
				AmountMicros: amountMicros,
				RelatedGuid:  relation.Guid,
				Remark:       code.Code,
			}); err != nil {
				return err
			}
		}
		if code.InviteeRewardAmount > 0 {
			amountMicros := WholeAmountToMicros(code.InviteeRewardAmount)
			if req.TokenID > 0 {
				if err := TokenServiceApp.AddAmount(tx, req.TokenID, userGuid, amountMicros); err != nil {
					return err
				}
			}
			if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
				UserGuid:     userGuid,
				Type:         domains.WalletRecordTypeReward,
				Source:       domains.WalletSourceInvitation,
				Title:        "邀请注册奖励",
				AmountMicros: amountMicros,
				TokenID:      req.TokenID,
				RelatedGuid:  relation.Guid,
				Remark:       code.Code,
			}); err != nil {
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

func (s *InvitationService) Stats(userGuid string) (map[string]any, error) {
	result := map[string]any{}
	codeDB := s.CodeCrud.DB().Model(&domains.InvitationCode{})
	relDB := s.RelationCrud.DB().Model(&domains.InvitationRelation{})
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
		RewardAmount        int64
		InviteeRewardAmount int64
	}
	if err := relDB.Select("COALESCE(SUM(reward_amount),0) AS reward_amount, COALESCE(SUM(invitee_reward_amount),0) AS invitee_reward_amount").Scan(&sums).Error; err != nil {
		return nil, err
	}
	result["totalCodes"] = totalCodes
	result["totalInvites"] = totalRelations
	result["rewardAmount"] = sums.RewardAmount
	result["inviteeRewardAmount"] = sums.InviteeRewardAmount
	return result, nil
}

func (s *InvitationService) newCode() (string, error) {
	raw, err := randomHex(6)
	if err != nil {
		return "", err
	}
	return "INV-" + strings.ToUpper(raw), nil
}
