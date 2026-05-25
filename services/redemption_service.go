package services

import (
	"errors"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RedemptionService struct{}

var RedemptionServiceApp = RedemptionService{}

func (s RedemptionService) Create(redemption *domains.Redemption) error {
	if redemption.Status == 0 {
		redemption.Status = constants.StatusEnabled
	}
	return global.NAV_DB.Create(redemption).Error
}

func (s RedemptionService) Update(redemption *domains.Redemption) error {
	return global.NAV_DB.Save(redemption).Error
}

func (s RedemptionService) Delete(id uint) error {
	return global.NAV_DB.Delete(&domains.Redemption{}, id).Error
}

func (s RedemptionService) List(query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var redemptions []domains.Redemption
	var total int64
	db := global.NAV_DB.Model(&domains.Redemption{})
	if query.Q != "" {
		db = db.Where("code LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&redemptions).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: redemptions, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s RedemptionService) Redeem(code string, userGuid string, tokenID uint) (*domains.Redemption, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	if tokenID == 0 {
		return nil, errors.New("token id is required")
	}
	var redeemed domains.Redemption
	err := global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		var redemption domains.Redemption
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&redemption).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		if redemption.Status != constants.StatusEnabled {
			return errors.New("redemption is disabled or used")
		}
		if redemption.ExpiredAt > 0 && redemption.ExpiredAt < now {
			return errors.New("redemption is expired")
		}
		if redemption.Quota <= 0 {
			return errors.New("redemption quota must be greater than zero")
		}
		if err := TokenServiceApp.AddQuota(tx, tokenID, userGuid, redemption.Quota); err != nil {
			return err
		}
		redemption.Status = constants.StatusDisabled
		redemption.UsedBy = userGuid
		redemption.UsedAt = now
		if err := tx.Save(&redemption).Error; err != nil {
			return err
		}
		redeemed = redemption
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &redeemed, nil
}
