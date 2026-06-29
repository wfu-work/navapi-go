package services

import (
	"errors"
	"fmt"
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

type RedemptionBatchRequest struct {
	Count     int    `json:"count"`
	Quota     int64  `json:"quota"`
	ExpiredAt int64  `json:"expiredAt"`
	Prefix    string `json:"prefix"`
	Remark    string `json:"remark"`
}

type RedemptionStats struct {
	Total      int64 `json:"total"`
	Enabled    int64 `json:"enabled"`
	Used       int64 `json:"used"`
	Expired    int64 `json:"expired"`
	TotalQuota int64 `json:"totalQuota"`
	UsedQuota  int64 `json:"usedQuota"`
}

func (s RedemptionService) Create(redemption *domains.Redemption) error {
	if redemption.Status == 0 {
		redemption.Status = constants.StatusEnabled
	}
	if redemption.Code == "" {
		code, err := s.newCode("")
		if err != nil {
			return err
		}
		redemption.Code = code
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

func (s RedemptionService) BatchCreate(req RedemptionBatchRequest) ([]domains.Redemption, error) {
	if req.Count <= 0 {
		return nil, errors.New("count must be greater than zero")
	}
	if req.Count > 1000 {
		return nil, errors.New("count cannot exceed 1000")
	}
	if req.Quota <= 0 {
		return nil, errors.New("quota must be greater than zero")
	}
	cards := make([]domains.Redemption, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code, err := s.newCode(req.Prefix)
		if err != nil {
			return nil, err
		}
		cards = append(cards, domains.Redemption{
			Code:      code,
			Quota:     req.Quota,
			Status:    constants.StatusEnabled,
			ExpiredAt: req.ExpiredAt,
			Remark:    req.Remark,
		})
	}
	if err := global.NAV_DB.Create(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

func (s RedemptionService) Stats() (RedemptionStats, error) {
	var stats RedemptionStats
	now := time.Now().Unix()
	if err := global.NAV_DB.Model(&domains.Redemption{}).Count(&stats.Total).Error; err != nil {
		return stats, err
	}
	if err := global.NAV_DB.Model(&domains.Redemption{}).Where("status = ?", constants.StatusEnabled).Count(&stats.Enabled).Error; err != nil {
		return stats, err
	}
	if err := global.NAV_DB.Model(&domains.Redemption{}).Where("used_at > 0 OR used_by <> ''").Count(&stats.Used).Error; err != nil {
		return stats, err
	}
	if err := global.NAV_DB.Model(&domains.Redemption{}).Where("expired_at > 0 AND expired_at < ?", now).Count(&stats.Expired).Error; err != nil {
		return stats, err
	}
	var sums struct {
		TotalQuota int64
		UsedQuota  int64
	}
	if err := global.NAV_DB.Model(&domains.Redemption{}).
		Select("COALESCE(SUM(quota),0) AS total_quota, COALESCE(SUM(CASE WHEN used_at > 0 OR used_by <> '' THEN quota ELSE 0 END),0) AS used_quota").
		Scan(&sums).Error; err != nil {
		return stats, err
	}
	stats.TotalQuota = sums.TotalQuota
	stats.UsedQuota = sums.UsedQuota
	return stats, nil
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

func (s RedemptionService) newCode(prefix string) (string, error) {
	raw, err := randomHex(10)
	if err != nil {
		return "", err
	}
	if prefix = normalizeCardPrefix(prefix); prefix != "" {
		return fmt.Sprintf("%s-%s", prefix, raw), nil
	}
	return raw, nil
}

func normalizeCardPrefix(prefix string) string {
	out := ""
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z':
			out += string(r - 32)
		case r >= 'A' && r <= 'Z':
			out += string(r)
		case r >= '0' && r <= '9':
			out += string(r)
		}
	}
	if len(out) > 16 {
		return out[:16]
	}
	return out
}
