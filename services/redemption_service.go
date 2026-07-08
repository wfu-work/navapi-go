package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"
)

type RedemptionService struct {
	commonServices.CrudService[domains.Redemption]
}

var RedemptionServiceApp = new(RedemptionService)

func (s *RedemptionService) WithDB(db *gorm.DB) *RedemptionService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type RedemptionBatchRequest struct {
	Count     int    `json:"count"`
	Amount    int64  `json:"amount"`
	ExpiredAt int64  `json:"expiredAt"`
	Prefix    string `json:"prefix"`
	Remark    string `json:"remark"`
}

type RedemptionStats struct {
	Total       int64 `json:"total"`
	Enabled     int64 `json:"enabled"`
	Used        int64 `json:"used"`
	Expired     int64 `json:"expired"`
	TotalAmount int64 `json:"totalAmount"`
	UsedAmount  int64 `json:"usedAmount"`
}

func (s *RedemptionService) Create(redemption *domains.Redemption) error {
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
	return createWithCrud(&s.CrudService, redemption)
}

func (s *RedemptionService) Update(redemption *domains.Redemption) error {
	redemption.Guid = strings.TrimSpace(redemption.Guid)
	if redemption.Guid == "" {
		return errors.New("guid is required")
	}
	existing, err := s.GetByGuid(redemption.Guid)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("redemption not found")
	}
	// 管理端编辑只用业务 GUID 定位，避免对外暴露数据库自增 ID。
	redemption.Guid = existing.Guid
	redemption.CreateTime = existing.CreateTime
	redemption.Creater = existing.Creater
	updating := *redemption
	updating.Id = 0
	if err := createWithCrud(&s.CrudService, &updating); err != nil {
		return err
	}
	*redemption = updating
	return nil
}

func (s *RedemptionService) Delete(guid string) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return errors.New("guid is required")
	}
	existing, err := s.GetByGuid(guid)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("redemption not found")
	}
	return s.DeleteByGuid(guid)
}

func (s *RedemptionService) Get(guid string) (*domains.Redemption, error) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil, errors.New("guid is required")
	}
	redemption, err := s.GetByGuid(guid)
	if err != nil {
		return nil, err
	}
	if redemption == nil {
		return nil, errors.New("redemption not found")
	}
	return redemption, nil
}

func (s *RedemptionService) List(query vos.PageQuery) (vos.PageResult, error) {
	query.Normalize()
	var redemptions []domains.Redemption
	var total int64
	db := s.DB().Model(&domains.Redemption{})
	if query.Q != "" {
		db = db.Where("code LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&redemptions).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: redemptions, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *RedemptionService) BatchCreate(req RedemptionBatchRequest) ([]domains.Redemption, error) {
	if req.Count <= 0 {
		return nil, errors.New("count must be greater than zero")
	}
	if req.Count > 1000 {
		return nil, errors.New("count cannot exceed 1000")
	}
	if req.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}
	cards := make([]domains.Redemption, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code, err := s.newCode(req.Prefix)
		if err != nil {
			return nil, err
		}
		cards = append(cards, domains.Redemption{
			Code:      code,
			Amount:    req.Amount,
			Status:    constants.StatusEnabled,
			ExpiredAt: req.ExpiredAt,
			Remark:    req.Remark,
		})
	}
	if err := s.DB().Create(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}

func (s *RedemptionService) Stats() (RedemptionStats, error) {
	var stats RedemptionStats
	now := time.Now().Unix()
	if err := s.DB().Model(&domains.Redemption{}).Count(&stats.Total).Error; err != nil {
		return stats, err
	}
	if err := s.DB().Model(&domains.Redemption{}).Where("status = ?", constants.StatusEnabled).Count(&stats.Enabled).Error; err != nil {
		return stats, err
	}
	if err := s.DB().Model(&domains.Redemption{}).Where("used_at > 0 OR used_by <> ''").Count(&stats.Used).Error; err != nil {
		return stats, err
	}
	if err := s.DB().Model(&domains.Redemption{}).Where("expired_at > 0 AND expired_at < ?", now).Count(&stats.Expired).Error; err != nil {
		return stats, err
	}
	var sums struct {
		TotalAmount int64
		UsedAmount  int64
	}
	if err := s.DB().Model(&domains.Redemption{}).
		Select("COALESCE(SUM(amount),0) AS total_amount, COALESCE(SUM(CASE WHEN used_at > 0 OR used_by <> '' THEN amount ELSE 0 END),0) AS used_amount").
		Scan(&sums).Error; err != nil {
		return stats, err
	}
	stats.TotalAmount = sums.TotalAmount
	stats.UsedAmount = sums.UsedAmount
	return stats, nil
}

func (s *RedemptionService) Redeem(code string, userGuid string) (*domains.Redemption, error) {
	code = strings.TrimSpace(code)
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	if code == "" {
		return nil, errors.New("redemption code is required")
	}
	var redeemed domains.Redemption
	err := s.DB().Transaction(func(tx *gorm.DB) error {
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
		if redemption.Amount <= 0 {
			return errors.New("redemption amount must be greater than zero")
		}
		amountMicros := WholeAmountToMicros(redemption.Amount)
		// 卡密只给用户钱包入账，接口调用余额统一由钱包结算。
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid:     userGuid,
			Type:         domains.WalletRecordTypeRecharge,
			Source:       domains.WalletSourceRedemption,
			Title:        "卡密兑换",
			AmountMicros: amountMicros,
			Currency:     "CNY",
			RelatedGuid:  redemption.Guid,
			Remark:       redemption.Code,
			OccurredAt:   now,
		}); err != nil {
			return err
		}
		redemption.Status = constants.StatusDisabled
		redemption.UsedBy = userGuid
		redemption.UsedAt = now
		redemption.UpdateTime = time.Now().UnixMilli()
		if err := tx.Model(&domains.Redemption{}).Where("id = ?", redemption.Id).Updates(map[string]any{
			"status":      redemption.Status,
			"used_by":     redemption.UsedBy,
			"used_at":     redemption.UsedAt,
			"update_time": redemption.UpdateTime,
		}).Error; err != nil {
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

func (s *RedemptionService) newCode(prefix string) (string, error) {
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
