package services

import (
	"errors"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
)

type SubscriptionService struct{}

var SubscriptionServiceApp = SubscriptionService{}

type SubscribeRequest struct {
	PlanID  uint   `json:"planId"`
	TokenID uint   `json:"tokenId"`
	Remark  string `json:"remark"`
}

func (s SubscriptionService) ListPlans(query dto.PageQuery, enabledOnly bool) (dto.PageResult, error) {
	query.Normalize()
	var plans []domains.SubscriptionPlan
	var total int64
	db := global.NAV_DB.Model(&domains.SubscriptionPlan{})
	if enabledOnly {
		db = db.Where("status = ?", constants.StatusEnabled)
	}
	if query.Q != "" {
		db = db.Where("name LIKE ? OR code LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("sort desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&plans).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: plans, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s SubscriptionService) SavePlan(plan *domains.SubscriptionPlan) error {
	if strings.TrimSpace(plan.Name) == "" {
		return errors.New("plan name is required")
	}
	if strings.TrimSpace(plan.Code) == "" {
		return errors.New("plan code is required")
	}
	if plan.Status == 0 {
		plan.Status = constants.StatusEnabled
	}
	if plan.DurationDays <= 0 {
		plan.DurationDays = 30
	}
	if plan.Currency == "" {
		plan.Currency = "CNY"
	}
	if plan.Group == "" {
		plan.Group = constants.DefaultGroup
	}
	return global.NAV_DB.Save(plan).Error
}

func (s SubscriptionService) DeletePlan(id uint) error {
	return global.NAV_DB.Delete(&domains.SubscriptionPlan{}, id).Error
}

func (s SubscriptionService) GetPlan(id uint) (*domains.SubscriptionPlan, error) {
	var plan domains.SubscriptionPlan
	if err := global.NAV_DB.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s SubscriptionService) ListUserSubscriptions(userGuid string, query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var subscriptions []domains.UserSubscription
	var total int64
	db := global.NAV_DB.Model(&domains.UserSubscription{})
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		db = db.Where("plan_name LIKE ? OR plan_code LIKE ? OR status LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&subscriptions).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: subscriptions, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s SubscriptionService) Subscribe(userGuid string, req SubscribeRequest, paymentGuid string) (*domains.UserSubscription, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	plan, err := s.GetPlan(req.PlanID)
	if err != nil {
		return nil, err
	}
	if plan.Status != constants.StatusEnabled {
		return nil, errors.New("subscription plan is disabled")
	}
	if plan.PriceCents > 0 && paymentGuid == "" {
		return nil, errors.New("paid subscription must be activated by payment")
	}
	var subscription domains.UserSubscription
	err = global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		created, err := s.createSubscriptionWithTx(tx, userGuid, plan, paymentGuid, req.Remark)
		if err != nil {
			return err
		}
		if err := UserQuotaServiceApp.Recharge(tx, userGuid, req.TokenID, plan.Quota); err != nil {
			return err
		}
		subscription = *created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (s SubscriptionService) createSubscriptionWithTx(tx *gorm.DB, userGuid string, plan *domains.SubscriptionPlan, paymentGuid string, remark string) (*domains.UserSubscription, error) {
	now := time.Now().Unix()
	subscription := domains.UserSubscription{
		UserGuid:    userGuid,
		PlanGuid:    plan.Guid,
		PlanCode:    plan.Code,
		PlanName:    plan.Name,
		Status:      "active",
		Quota:       plan.Quota,
		StartAt:     now,
		EndAt:       now + int64(plan.DurationDays)*86400,
		PaymentGuid: paymentGuid,
		Remark:      remark,
	}
	if err := tx.Create(&subscription).Error; err != nil {
		return nil, err
	}
	return &subscription, nil
}
