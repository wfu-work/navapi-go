package services

import (
	"errors"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserQuotaService struct{}

var UserQuotaServiceApp = UserQuotaService{}

func (s UserQuotaService) Ensure(tx *gorm.DB, userGuid string) error {
	if userGuid == "" {
		return nil
	}
	account := domains.UserQuota{UserGuid: userGuid, Group: constants.DefaultGroup}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error
}

func (s UserQuotaService) Get(userGuid string) (*domains.UserQuota, error) {
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	var account domains.UserQuota
	err := global.NAV_DB.Where("user_guid = ?", userGuid).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := global.NAV_DB.Transaction(func(tx *gorm.DB) error {
			return s.Ensure(tx, userGuid)
		}); err != nil {
			return nil, err
		}
		err = global.NAV_DB.Where("user_guid = ?", userGuid).First(&account).Error
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s UserQuotaService) List(query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var accounts []domains.UserQuota
	var total int64
	db := global.NAV_DB.Model(&domains.UserQuota{})
	if query.Q != "" {
		db = db.Where("user_guid LIKE ? OR group_name LIKE ? OR allowed_groups LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&accounts).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: accounts, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s UserQuotaService) Update(account *domains.UserQuota) error {
	if account.UserGuid == "" {
		return errors.New("user guid is required")
	}
	if account.Group == "" {
		account.Group = constants.DefaultGroup
	}
	return global.NAV_DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_guid"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"remain_quota",
			"used_quota",
			"total_quota",
			"group_name",
			"allowed_groups",
		}),
	}).Create(account).Error
}

func (s UserQuotaService) AddQuota(tx *gorm.DB, userGuid string, quota int64) error {
	if userGuid == "" || quota <= 0 {
		return nil
	}
	if err := s.Ensure(tx, userGuid); err != nil {
		return err
	}
	return tx.Model(&domains.UserQuota{}).Where("user_guid = ?", userGuid).
		Updates(map[string]any{
			"remain_quota": gorm.Expr("remain_quota + ?", quota),
			"total_quota":  gorm.Expr("total_quota + ?", quota),
		}).Error
}

func (s UserQuotaService) Consume(tx *gorm.DB, userGuid string, quota int64) error {
	if userGuid == "" || quota <= 0 {
		return nil
	}
	if err := s.Ensure(tx, userGuid); err != nil {
		return err
	}
	return tx.Model(&domains.UserQuota{}).Where("user_guid = ?", userGuid).
		Updates(map[string]any{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		}).Error
}

func (s UserQuotaService) CheckGroup(userGuid string, group string) error {
	if userGuid == "" {
		return nil
	}
	account, err := s.Get(userGuid)
	if err != nil {
		return err
	}
	if account.AllowedGroups == "" {
		return nil
	}
	if containsString(splitCSV(account.AllowedGroups), normalizeGroup(group)) {
		return nil
	}
	return errors.New("group is not allowed for user")
}
