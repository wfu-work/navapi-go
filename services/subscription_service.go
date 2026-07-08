package services

import (
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
	"unicode"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"
)

type SubscriptionService struct {
	commonServices.CrudService[domains.SubscriptionPlan]
	UserSubscriptionCrud commonServices.CrudService[domains.UserSubscription]
}

var SubscriptionServiceApp = new(SubscriptionService)

func (s *SubscriptionService) WithDB(db *gorm.DB) *SubscriptionService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	cloned.UserSubscriptionCrud = *s.UserSubscriptionCrud.WithDB(db)
	return &cloned
}

type SubscribeRequest struct {
	PlanID  uint   `json:"planId"`
	TokenID uint   `json:"tokenId"`
	Remark  string `json:"remark"`
}

func (s *SubscriptionService) ListPlans(query vos.PageQuery, enabledOnly bool) (vos.PageResult, error) {
	query.Normalize()
	var plans []domains.SubscriptionPlan
	var total int64
	db := s.DB().Model(&domains.SubscriptionPlan{})
	if enabledOnly {
		db = db.Where("status = ?", constants.StatusEnabled)
	}
	if query.Q != "" {
		db = db.Where("name LIKE ? OR code LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("sort desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&plans).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: plans, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *SubscriptionService) SavePlan(plan *domains.SubscriptionPlan) error {
	plan.Name = strings.TrimSpace(plan.Name)
	plan.Code = normalizeSubscriptionPlanCode(plan.Code)
	if plan.Name == "" {
		return errors.New("plan name is required")
	}
	if plan.Status == 0 {
		plan.Status = constants.StatusEnabled
	}
	if plan.DurationDays <= 0 {
		plan.DurationDays = 30
	}
	if plan.WeeklyAmount < 0 {
		plan.WeeklyAmount = 0
	}
	if plan.Amount < 0 {
		plan.Amount = 0
	}
	if plan.Amount <= 0 {
		return errors.New("plan amount must be greater than zero")
	}
	if plan.Currency == "" {
		plan.Currency = "CNY"
	}
	if plan.Group == "" {
		plan.Group = constants.DefaultGroup
	}
	var existing *domains.SubscriptionPlan
	if plan.Id != 0 {
		var err error
		existing, err = s.GetById(plan.Id)
		if err != nil {
			return err
		}
		if existing == nil {
			return errors.New("subscription plan not found")
		}
	}
	if plan.Code == "" && existing != nil {
		plan.Code = existing.Code
	}
	if plan.Code == "" {
		plan.Code = subscriptionPlanCodeFromName(plan.Name)
	}
	if plan.Id == 0 {
		return createWithCrud(&s.CrudService, plan)
	}
	plan.Guid = existing.Guid
	plan.CreateTime = existing.CreateTime
	plan.Creater = existing.Creater
	updating := *plan
	updating.Id = 0
	if err := createWithCrud(&s.CrudService, &updating); err != nil {
		return err
	}
	*plan = updating
	return nil
}

func normalizeSubscriptionPlanCode(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return subscriptionPlanCodeFromName(value)
}

func subscriptionPlanCodeFromName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	pendingDash := false
	for _, item := range value {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			if pendingDash && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(item)
			pendingDash = false
			continue
		}
		pendingDash = builder.Len() > 0
	}
	code := strings.Trim(builder.String(), "-")
	if code != "" {
		return code
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(value))
	return "plan-" + strconv.FormatUint(uint64(hash.Sum32()), 36)
}

func (s *SubscriptionService) DeletePlan(id uint) error {
	return deleteByIDWithCrud(&s.CrudService, id, "subscription plan not found")
}

func (s *SubscriptionService) GetPlan(id uint) (*domains.SubscriptionPlan, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	plan, err := s.GetById(id)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, errors.New("subscription plan not found")
	}
	return plan, nil
}

func (s *SubscriptionService) ListUserSubscriptions(userGuid string, query vos.PageQuery) (vos.PageResult, error) {
	query.Normalize()
	var subscriptions []domains.UserSubscription
	var total int64
	db := s.UserSubscriptionCrud.DB().Model(&domains.UserSubscription{})
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		db = db.Where(
			"user_guid LIKE ? OR plan_name LIKE ? OR plan_code LIKE ? OR status LIKE ? OR payment_guid LIKE ? OR remark LIKE ?",
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
		)
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&subscriptions).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: subscriptions, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *SubscriptionService) Subscribe(userGuid string, req SubscribeRequest, paymentGuid string) (*domains.UserSubscription, error) {
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
	err = s.DB().Transaction(func(tx *gorm.DB) error {
		created, err := s.createSubscriptionWithTx(tx, userGuid, plan, paymentGuid, req.Remark)
		if err != nil {
			return err
		}
		amountMicros := WholeAmountToMicros(plan.Amount)
		if amountMicros > 0 {
			if req.TokenID > 0 {
				if err := TokenServiceApp.AddAmount(tx, req.TokenID, userGuid, amountMicros); err != nil {
					return err
				}
			}
			if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
				UserGuid:         userGuid,
				Type:             domains.WalletRecordTypeSubscription,
				Source:           domains.WalletSourceSubscription,
				Title:            "订阅开通",
				AmountMicros:     amountMicros,
				AmountCents:      plan.PriceCents,
				Currency:         plan.Currency,
				PaymentGuid:      paymentGuid,
				SubscriptionGuid: created.Guid,
				TokenID:          req.TokenID,
				RelatedGuid:      plan.Guid,
				Remark:           req.Remark,
			}); err != nil {
				return err
			}
		}
		subscription = *created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (s *SubscriptionService) createSubscriptionWithTx(tx *gorm.DB, userGuid string, plan *domains.SubscriptionPlan, paymentGuid string, remark string) (*domains.UserSubscription, error) {
	now := time.Now().Unix()
	subscription := domains.UserSubscription{
		UserGuid:     userGuid,
		PlanGuid:     plan.Guid,
		PlanCode:     plan.Code,
		PlanName:     plan.Name,
		Status:       "active",
		WeeklyAmount: plan.WeeklyAmount,
		Amount:       plan.Amount,
		StartAt:      now,
		EndAt:        now + int64(plan.DurationDays)*86400,
		PaymentGuid:  paymentGuid,
		Remark:       remark,
	}
	if err := subscription.BeforeCreate(nil); err != nil {
		return nil, err
	}
	userSubscriptionCrud := s.UserSubscriptionCrud.WithDB(tx)
	if err := userSubscriptionCrud.Create(subscription); err != nil {
		return nil, err
	}
	return &subscription, nil
}
