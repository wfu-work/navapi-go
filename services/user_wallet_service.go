package services

import (
	"errors"
	"strings"
	"time"

	"navapi-go/domains"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserWalletService struct {
	commonServices.CrudService[domains.UserWallet]
	RecordCrud commonServices.CrudService[domains.UserWalletRecord]
}

var UserWalletServiceApp = new(UserWalletService)

type WalletRecordInput struct {
	UserGuid         string
	Type             string
	Source           string
	Title            string
	Quota            int64
	RequestCount     int64
	AmountCents      int64
	Currency         string
	OrderNo          string
	PaymentGuid      string
	SubscriptionGuid string
	TokenID          uint
	TokenGuid        string
	RelatedGuid      string
	Remark           string
	Meta             string
	OccurredAt       int64
}

func (s *UserWalletService) WithDB(db *gorm.DB) *UserWalletService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	cloned.RecordCrud = *s.RecordCrud.WithDB(db)
	return &cloned
}

func (s *UserWalletService) Ensure(tx *gorm.DB, userGuid string) error {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil
	}
	wallet := domains.UserWallet{
		UserGuid: userGuid,
		Currency: "CNY",
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&wallet).Error
}

func (s *UserWalletService) ensureFromQuota(tx *gorm.DB, userGuid string) error {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil
	}
	wallet := domains.UserWallet{
		UserGuid: userGuid,
		Currency: "CNY",
	}
	var quota domains.UserQuota
	err := tx.Where("user_guid = ?", userGuid).First(&quota).Error
	if err == nil {
		wallet.BalanceQuota = nonNegativeInt64(quota.RemainQuota)
		wallet.PaidBalanceQuota = wallet.BalanceQuota
		wallet.TotalRechargeQuota = nonNegativeInt64(quota.TotalQuota)
		wallet.TotalConsumedQuota = nonNegativeInt64(quota.UsedQuota)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&wallet).Error
}

func (s *UserWalletService) Get(userGuid string) (*domains.UserWallet, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	if err := s.DB().Transaction(func(tx *gorm.DB) error {
		return s.ensureFromQuota(tx, userGuid)
	}); err != nil {
		return nil, err
	}
	var wallet domains.UserWallet
	if err := s.DB().Where("user_guid = ?", userGuid).First(&wallet).Error; err != nil {
		return nil, err
	}
	normalizeWallet(&wallet)
	return &wallet, nil
}

func (s *UserWalletService) RecordIncome(tx *gorm.DB, input WalletRecordInput) error {
	input = normalizeWalletRecordInput(input)
	if input.UserGuid == "" {
		return nil
	}
	if input.Quota <= 0 && input.AmountCents <= 0 {
		return nil
	}
	if err := s.Ensure(tx, input.UserGuid); err != nil {
		return err
	}
	wallet, err := s.lockWallet(tx, input.UserGuid)
	if err != nil {
		return err
	}
	applyWalletIncome(wallet, input)
	if err := s.saveWallet(tx, wallet); err != nil {
		return err
	}
	return s.createRecord(tx, wallet, input, domains.WalletRecordDirectionIncome, input.Quota)
}

func (s *UserWalletService) RecordConsume(tx *gorm.DB, input WalletRecordInput) error {
	input = normalizeWalletRecordInput(input)
	if input.UserGuid == "" {
		return nil
	}
	if input.Quota <= 0 && input.RequestCount <= 0 {
		return nil
	}
	if err := s.Ensure(tx, input.UserGuid); err != nil {
		return err
	}
	wallet, err := s.lockWallet(tx, input.UserGuid)
	if err != nil {
		return err
	}
	quota := nonNegativeInt64(input.Quota)
	requestCount := nonNegativeInt64(input.RequestCount)
	deductWalletQuota(wallet, quota)
	wallet.TotalConsumedQuota += quota
	wallet.TotalRequestCount += requestCount
	if err := s.saveWallet(tx, wallet); err != nil {
		return err
	}
	input.Type = domains.WalletRecordTypeConsume
	input.Source = defaultString(input.Source, domains.WalletSourceRelay)
	input.RequestCount = requestCount
	return s.createRecord(tx, wallet, input, domains.WalletRecordDirectionOutcome, -quota)
}

func (s *UserWalletService) lockWallet(tx *gorm.DB, userGuid string) (*domains.UserWallet, error) {
	var wallet domains.UserWallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_guid = ?", userGuid).First(&wallet).Error; err != nil {
		return nil, err
	}
	normalizeWallet(&wallet)
	return &wallet, nil
}

func (s *UserWalletService) saveWallet(tx *gorm.DB, wallet *domains.UserWallet) error {
	normalizeWallet(wallet)
	return tx.Model(&domains.UserWallet{}).Where("id = ?", wallet.Id).Updates(map[string]any{
		"balance_quota":                   wallet.BalanceQuota,
		"paid_balance_quota":              wallet.PaidBalanceQuota,
		"reward_balance_quota":            wallet.RewardBalanceQuota,
		"commission_balance_quota":        wallet.CommissionBalanceQuota,
		"total_consumed_quota":            wallet.TotalConsumedQuota,
		"total_request_count":             wallet.TotalRequestCount,
		"total_recharge_quota":            wallet.TotalRechargeQuota,
		"total_subscription_quota":        wallet.TotalSubscriptionQuota,
		"total_reward_quota":              wallet.TotalRewardQuota,
		"total_commission_quota":          wallet.TotalCommissionQuota,
		"total_recharge_amount_cents":     wallet.TotalRechargeAmountCents,
		"total_subscription_amount_cents": wallet.TotalSubscriptionAmountCents,
		"currency":                        wallet.Currency,
	}).Error
}

func (s *UserWalletService) createRecord(tx *gorm.DB, wallet *domains.UserWallet, input WalletRecordInput, direction string, quotaDelta int64) error {
	record := domains.UserWalletRecord{
		UserGuid:               input.UserGuid,
		Type:                   input.Type,
		Direction:              direction,
		Source:                 input.Source,
		Title:                  input.Title,
		QuotaDelta:             quotaDelta,
		RequestCountDelta:      input.RequestCount,
		BalanceAfter:           wallet.BalanceQuota,
		PaidBalanceAfter:       wallet.PaidBalanceQuota,
		RewardBalanceAfter:     wallet.RewardBalanceQuota,
		CommissionBalanceAfter: wallet.CommissionBalanceQuota,
		AmountCents:            input.AmountCents,
		Currency:               input.Currency,
		OrderNo:                input.OrderNo,
		PaymentGuid:            input.PaymentGuid,
		SubscriptionGuid:       input.SubscriptionGuid,
		TokenID:                input.TokenID,
		TokenGuid:              input.TokenGuid,
		RelatedGuid:            input.RelatedGuid,
		OccurredAt:             input.OccurredAt,
		Remark:                 input.Remark,
		Meta:                   input.Meta,
	}
	if record.OccurredAt == 0 {
		record.OccurredAt = time.Now().Unix()
	}
	if err := record.BeforeCreate(nil); err != nil {
		return err
	}
	return tx.Create(&record).Error
}

func applyWalletIncome(wallet *domains.UserWallet, input WalletRecordInput) {
	quota := nonNegativeInt64(input.Quota)
	wallet.BalanceQuota += quota
	switch input.Type {
	case domains.WalletRecordTypeSubscription:
		wallet.PaidBalanceQuota += quota
		wallet.TotalSubscriptionQuota += quota
		wallet.TotalSubscriptionAmountCents += nonNegativeInt64(input.AmountCents)
	case domains.WalletRecordTypeReward:
		wallet.RewardBalanceQuota += quota
		wallet.TotalRewardQuota += quota
	case domains.WalletRecordTypeCommission:
		wallet.CommissionBalanceQuota += quota
		wallet.TotalCommissionQuota += quota
	default:
		wallet.PaidBalanceQuota += quota
		wallet.TotalRechargeQuota += quota
		wallet.TotalRechargeAmountCents += nonNegativeInt64(input.AmountCents)
	}
}

func deductWalletQuota(wallet *domains.UserWallet, quota int64) {
	remaining := nonNegativeInt64(quota)
	take := int64Min(wallet.RewardBalanceQuota, remaining)
	wallet.RewardBalanceQuota -= take
	remaining -= take
	take = int64Min(wallet.CommissionBalanceQuota, remaining)
	wallet.CommissionBalanceQuota -= take
	remaining -= take
	take = int64Min(wallet.PaidBalanceQuota, remaining)
	wallet.PaidBalanceQuota -= take
	wallet.BalanceQuota = wallet.PaidBalanceQuota + wallet.RewardBalanceQuota + wallet.CommissionBalanceQuota
}

func normalizeWallet(wallet *domains.UserWallet) {
	wallet.UserGuid = strings.TrimSpace(wallet.UserGuid)
	wallet.PaidBalanceQuota = nonNegativeInt64(wallet.PaidBalanceQuota)
	wallet.RewardBalanceQuota = nonNegativeInt64(wallet.RewardBalanceQuota)
	wallet.CommissionBalanceQuota = nonNegativeInt64(wallet.CommissionBalanceQuota)
	wallet.BalanceQuota = wallet.PaidBalanceQuota + wallet.RewardBalanceQuota + wallet.CommissionBalanceQuota
	wallet.TotalConsumedQuota = nonNegativeInt64(wallet.TotalConsumedQuota)
	wallet.TotalRequestCount = nonNegativeInt64(wallet.TotalRequestCount)
	wallet.TotalRechargeQuota = nonNegativeInt64(wallet.TotalRechargeQuota)
	wallet.TotalSubscriptionQuota = nonNegativeInt64(wallet.TotalSubscriptionQuota)
	wallet.TotalRewardQuota = nonNegativeInt64(wallet.TotalRewardQuota)
	wallet.TotalCommissionQuota = nonNegativeInt64(wallet.TotalCommissionQuota)
	wallet.TotalRechargeAmountCents = nonNegativeInt64(wallet.TotalRechargeAmountCents)
	wallet.TotalSubscriptionAmountCents = nonNegativeInt64(wallet.TotalSubscriptionAmountCents)
	wallet.Currency = defaultString(strings.TrimSpace(wallet.Currency), "CNY")
}

func normalizeWalletRecordInput(input WalletRecordInput) WalletRecordInput {
	input.UserGuid = strings.TrimSpace(input.UserGuid)
	input.Type = strings.TrimSpace(input.Type)
	input.Source = strings.TrimSpace(input.Source)
	input.Title = strings.TrimSpace(input.Title)
	input.Currency = defaultString(strings.TrimSpace(input.Currency), "CNY")
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.PaymentGuid = strings.TrimSpace(input.PaymentGuid)
	input.SubscriptionGuid = strings.TrimSpace(input.SubscriptionGuid)
	input.TokenGuid = strings.TrimSpace(input.TokenGuid)
	input.RelatedGuid = strings.TrimSpace(input.RelatedGuid)
	input.Remark = strings.TrimSpace(input.Remark)
	input.Meta = strings.TrimSpace(input.Meta)
	if input.Type == "" {
		input.Type = domains.WalletRecordTypeRecharge
	}
	if input.Source == "" {
		input.Source = domains.WalletSourceManual
	}
	return input
}

func int64Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
