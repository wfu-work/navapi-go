package services

import (
	"errors"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"
)

type UserQuotaService struct {
	commonServices.CrudService[domains.UserQuota]
}

var UserQuotaServiceApp = new(UserQuotaService)

func (s *UserQuotaService) WithDB(db *gorm.DB) *UserQuotaService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *UserQuotaService) Ensure(tx *gorm.DB, userGuid string) error {
	if userGuid == "" {
		return nil
	}
	settings := RegisterSettingServiceApp.Get()
	group := settings.DefaultGroup
	if group == "" {
		group = constants.DefaultGroup
	}
	account := domains.UserQuota{
		UserGuid:      userGuid,
		RemainQuota:   settings.DefaultQuota,
		TotalQuota:    settings.DefaultQuota,
		Group:         group,
		AllowedGroups: settings.AllowedGroups,
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error
}

func (s *UserQuotaService) Get(userGuid string) (*domains.UserQuota, error) {
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	accounts, err := s.ListByFields(map[string]any{"userGuid": userGuid})
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		if err := s.DB().Transaction(func(tx *gorm.DB) error {
			return s.Ensure(tx, userGuid)
		}); err != nil {
			return nil, err
		}
		accounts, err = s.ListByFields(map[string]any{"userGuid": userGuid})
	}
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, errors.New("user quota not found")
	}
	return &accounts[0], nil
}

func (s *UserQuotaService) List(query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var accounts []domains.UserQuota
	var total int64
	db := s.DB().Model(&domains.UserQuota{})
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

func (s *UserQuotaService) Update(account *domains.UserQuota) error {
	if account.UserGuid == "" {
		return errors.New("user guid is required")
	}
	if account.Group == "" {
		account.Group = constants.DefaultGroup
	}
	return s.DB().Clauses(clause.OnConflict{
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

func (s *UserQuotaService) AddQuota(tx *gorm.DB, userGuid string, quota int64) error {
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

// Recharge adds quota to the user account and optionally to one API token.
// Payments and subscriptions both use this path so recharge accounting stays
// consistent regardless of the source.
func (s *UserQuotaService) Recharge(tx *gorm.DB, userGuid string, tokenID uint, quota int64) error {
	if quota <= 0 {
		return errors.New("quota must be greater than zero")
	}
	if tokenID > 0 {
		return TokenServiceApp.AddQuota(tx, tokenID, userGuid, quota)
	}
	return s.AddQuota(tx, userGuid, quota)
}

func (s *UserQuotaService) Consume(tx *gorm.DB, userGuid string, quota int64) error {
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

// Refund only rolls back used_quota because user quota currently acts as an
// aggregate account; token quota is the balance that is actually decremented.
func (s *UserQuotaService) Refund(tx *gorm.DB, userGuid string, quota int64) error {
	if userGuid == "" || quota <= 0 {
		return nil
	}
	if err := s.Ensure(tx, userGuid); err != nil {
		return err
	}
	return tx.Model(&domains.UserQuota{}).Where("user_guid = ?", userGuid).
		Update("used_quota", gorm.Expr("CASE WHEN used_quota >= ? THEN used_quota - ? ELSE 0 END", quota, quota)).Error
}

func (s *UserQuotaService) CheckGroup(userGuid string, group string) error {
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
