package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"navapi-go/domains"
	"navapi-go/vos"
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
	Type         string `json:"type"`
	TokenID      uint   `json:"tokenId"`
	AmountCents  int64  `json:"amountCents"`
	AmountMicros int64  `json:"amountMicros"`
	Currency     string `json:"currency"`
	PlanID       uint   `json:"planId"`
	Provider     string `json:"provider"`
	Remark       string `json:"remark"`
}

type ConfirmPaymentRequest struct {
	OrderNo       string `json:"orderNo"`
	TransactionID string `json:"transactionId"`
	NotifyData    string `json:"notifyData"`
}

func (s *PaymentService) List(userGuid string, query vos.PageQuery) (vos.PageResult, error) {
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
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&orders).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: orders, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *PaymentService) Create(userGuid string, req CreatePaymentRequest) (*domains.PaymentOrder, error) {
	return s.CreateWithContext(context.Background(), userGuid, req)
}

func (s *PaymentService) CreateWithContext(ctx context.Context, userGuid string, req CreatePaymentRequest) (*domains.PaymentOrder, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	order := domains.PaymentOrder{
		OrderNo:      newOrderNo(),
		UserGuid:     userGuid,
		TokenID:      req.TokenID,
		Type:         normalizePaymentType(req.Type),
		Status:       "pending",
		Provider:     normalizePaymentProvider(req.Provider),
		AmountCents:  req.AmountCents,
		AmountMicros: req.AmountMicros,
		Currency:     req.Currency,
		Remark:       req.Remark,
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
		order.AmountMicros = WholeAmountToMicros(plan.Amount)
		order.PlanGuid = plan.Guid
		order.PlanCode = plan.Code
		order.Currency = plan.Currency
	} else {
		if order.AmountMicros <= 0 {
			order.AmountMicros = AmountCentsToMicros(order.AmountCents)
		}
		if order.AmountMicros <= 0 {
			return nil, errors.New("amount must be greater than zero")
		}
	}
	if req.TokenID > 0 {
		token, err := TokenServiceApp.GetByID(req.TokenID, userGuid)
		if err != nil {
			return nil, err
		}
		order.TokenGuid = token.Guid
	}
	if isWechatPaymentProvider(order.Provider) {
		// 支付渠道未开放或配置不完整时直接拒绝创建，避免产生无效的失败订单记录。
		if err := s.validateWechatPaymentReady(); err != nil {
			return nil, err
		}
	}
	if err := createWithCrud(&s.CrudService, &order); err != nil {
		return nil, err
	}
	if isWechatPaymentProvider(order.Provider) {
		if err := s.createWechatNativePrepay(ctx, &order); err != nil {
			_ = s.DB().Model(&domains.PaymentOrder{}).Where("guid = ?", order.Guid).Updates(map[string]any{
				"status":      "closed",
				"closed_at":   time.Now().Unix(),
				"notify_data": "wechat prepay failed: " + err.Error(),
			}).Error
			return nil, err
		}
	}
	return &order, nil
}

// Confirm marks an order paid and applies the purchased balance/subscription.
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
		var subscription *domains.UserSubscription
		order.Status = "paid"
		order.PaidAt = now
		order.TransactionID = req.TransactionID
		order.NotifyData = req.NotifyData
		if order.AmountMicros <= 0 {
			order.AmountMicros = AmountCentsToMicros(order.AmountCents)
		}
		if order.AmountMicros <= 0 {
			return errors.New("amount must be greater than zero")
		}
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
			subscription, err = SubscriptionServiceApp.createSubscriptionWithTx(tx, order.UserGuid, plan, order.Guid, order.Remark)
			if err != nil {
				return err
			}
		}
		if order.TokenID > 0 {
			if err := TokenServiceApp.AddAmount(tx, order.TokenID, order.UserGuid, order.AmountMicros); err != nil {
				return err
			}
		}
		recordType := domains.WalletRecordTypeRecharge
		source := domains.WalletSourcePayment
		title := "充值"
		subscriptionGuid := ""
		relatedGuid := order.Guid
		if order.Type == "subscription" {
			recordType = domains.WalletRecordTypeSubscription
			source = domains.WalletSourceSubscription
			title = "订阅购买"
			if subscription != nil {
				subscriptionGuid = subscription.Guid
			}
			relatedGuid = order.PlanGuid
		}
		if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
			UserGuid:         order.UserGuid,
			Type:             recordType,
			Source:           source,
			Title:            title,
			AmountMicros:     order.AmountMicros,
			AmountCents:      order.AmountCents,
			Currency:         order.Currency,
			OrderNo:          order.OrderNo,
			PaymentGuid:      order.Guid,
			SubscriptionGuid: subscriptionGuid,
			TokenID:          order.TokenID,
			TokenGuid:        order.TokenGuid,
			RelatedGuid:      relatedGuid,
			Remark:           order.Remark,
			OccurredAt:       now,
		}); err != nil {
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
	return "recharge"
}

func normalizePaymentProvider(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "manual"
	}
	if isWechatPaymentProvider(value) {
		return paymentProviderWechat
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
