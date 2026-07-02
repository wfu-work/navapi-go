package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"navapi-go/domains"
	"navapi-go/dto"
)

type PaymentService struct {
	commonServices.CrudService[domains.PaymentOrder]
}

var PaymentServiceApp = new(PaymentService)

func (s *PaymentService) WithDB(db *gorm.DB) *PaymentService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type CreatePaymentRequest struct {
	Type        string `json:"type"`
	TokenID     uint   `json:"tokenId"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	Quota       int64  `json:"quota"`
	PlanID      uint   `json:"planId"`
	Provider    string `json:"provider"`
	Remark      string `json:"remark"`
}

type ConfirmPaymentRequest struct {
	OrderNo       string `json:"orderNo"`
	TransactionID string `json:"transactionId"`
	NotifyData    string `json:"notifyData"`
}

func (s *PaymentService) List(userGuid string, query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var orders []domains.PaymentOrder
	var total int64
	db := s.DB().Model(&domains.PaymentOrder{})
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		db = db.Where("order_no LIKE ? OR type LIKE ? OR status LIKE ? OR provider LIKE ? OR transaction_id LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&orders).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: orders, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *PaymentService) Create(userGuid string, req CreatePaymentRequest) (*domains.PaymentOrder, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	order := domains.PaymentOrder{
		OrderNo:     newOrderNo(),
		UserGuid:    userGuid,
		TokenID:     req.TokenID,
		Type:        normalizePaymentType(req.Type),
		Status:      "pending",
		Provider:    normalizePaymentProvider(req.Provider),
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Quota:       req.Quota,
		Remark:      req.Remark,
	}
	if order.Currency == "" {
		order.Currency = "CNY"
	}
	if order.Type == "subscription" {
		plan, err := SubscriptionServiceApp.GetPlan(req.PlanID)
		if err != nil {
			return nil, err
		}
		order.AmountCents = plan.PriceCents
		order.Quota = plan.Quota
		order.PlanGuid = plan.Guid
		order.PlanCode = plan.Code
		order.Currency = plan.Currency
	} else if order.Quota <= 0 {
		return nil, errors.New("quota must be greater than zero")
	}
	if req.TokenID > 0 {
		token, err := TokenServiceApp.GetByID(req.TokenID, userGuid)
		if err != nil {
			return nil, err
		}
		order.TokenGuid = token.Guid
	}
	if err := createWithCrud(&s.CrudService, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// Confirm marks an order paid and applies the purchased quota/subscription.
// External payment callbacks should verify signatures before calling this.
func (s *PaymentService) Confirm(req ConfirmPaymentRequest) (*domains.PaymentOrder, error) {
	if strings.TrimSpace(req.OrderNo) == "" {
		return nil, errors.New("orderNo is required")
	}
	var paid domains.PaymentOrder
	err := s.DB().Transaction(func(tx *gorm.DB) error {
		var order domains.PaymentOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", req.OrderNo).First(&order).Error; err != nil {
			return err
		}
		if order.Status == "paid" {
			paid = order
			return nil
		}
		if order.Status != "pending" {
			return errors.New("payment order is not pending")
		}
		now := time.Now().Unix()
		order.Status = "paid"
		order.PaidAt = now
		order.TransactionID = req.TransactionID
		order.NotifyData = req.NotifyData
		updating := order
		updating.Id = 0
		orderCrud := s.CrudService.WithDB(tx)
		if err := orderCrud.Create(updating); err != nil {
			return err
		}
		if order.Type == "subscription" {
			plan, err := findPlanByGuidOrCode(tx, order.PlanGuid, order.PlanCode)
			if err != nil {
				return err
			}
			if _, err := SubscriptionServiceApp.createSubscriptionWithTx(tx, order.UserGuid, plan, order.Guid, order.Remark); err != nil {
				return err
			}
		}
		if err := UserQuotaServiceApp.Recharge(tx, order.UserGuid, order.TokenID, order.Quota); err != nil {
			return err
		}
		paid = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &paid, nil
}

func (s *PaymentService) Close(orderNo string, userGuid string) error {
	if strings.TrimSpace(orderNo) == "" {
		return errors.New("orderNo is required")
	}
	return s.DB().Transaction(func(tx *gorm.DB) error {
		db := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo)
		if userGuid != "" {
			db = db.Where("user_guid = ?", userGuid)
		}
		var order domains.PaymentOrder
		if err := db.First(&order).Error; err != nil {
			return err
		}
		if order.Status != "pending" {
			return errors.New("only pending order can be closed")
		}
		return tx.Model(&domains.PaymentOrder{}).Where("id = ?", order.Id).Updates(map[string]any{
			"status":    "closed",
			"closed_at": time.Now().Unix(),
		}).Error
	})
}

func findPlanByGuidOrCode(tx *gorm.DB, guid string, code string) (*domains.SubscriptionPlan, error) {
	var plan domains.SubscriptionPlan
	db := tx
	if guid != "" {
		db = db.Where("guid = ?", guid)
	} else {
		db = db.Where("code = ?", code)
	}
	if err := db.First(&plan).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func normalizePaymentType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "subscription" {
		return value
	}
	return "quota"
}

func normalizePaymentProvider(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	return value
}

func newOrderNo() string {
	code, err := randomHex(8)
	if err != nil {
		return fmt.Sprintf("pay_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("pay_%d_%s", time.Now().Unix(), code)
}
