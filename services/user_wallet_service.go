package services

import (
	"errors"
	"strings"
	"time"

	"navapi-go/domains"
	"navapi-go/vos"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
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
	AmountMicros     int64
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

type WalletBalanceAccount struct {
	commonDomains.BaseDataEntity
	UserGuid                      string `json:"userGuid"`
	BalanceAmountMicros           int64  `json:"balanceAmountMicros"`
	PaidBalanceAmountMicros       int64  `json:"paidBalanceAmountMicros"`
	RewardBalanceAmountMicros     int64  `json:"rewardBalanceAmountMicros"`
	CommissionBalanceAmountMicros int64  `json:"commissionBalanceAmountMicros"`
	TotalConsumedAmountMicros     int64  `json:"totalConsumedAmountMicros"`
	TotalRequestCount             int64  `json:"totalRequestCount"`
	TotalRechargeAmountMicros     int64  `json:"totalRechargeAmountMicros"`
	TotalSubscriptionAmountMicros int64  `json:"totalSubscriptionAmountMicros"`
	TotalRewardAmountMicros       int64  `json:"totalRewardAmountMicros"`
	TotalCommissionAmountMicros   int64  `json:"totalCommissionAmountMicros"`
	TotalRechargeAmountCents      int64  `json:"totalRechargeAmountCents"`
	TotalSubscriptionAmountCents  int64  `json:"totalSubscriptionAmountCents"`
	Currency                      string `json:"currency"`

	RemainAmountMicros int64  `json:"remainAmountMicros"`
	UsedAmountMicros   int64  `json:"usedAmountMicros"`
	TotalAmountMicros  int64  `json:"totalAmountMicros"`
	Group              string `json:"group"`
	AllowedGroups      string `json:"allowedGroups"`
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

func (s *UserWalletService) EnsureWithInitialAmount(tx *gorm.DB, userGuid string, amountMicros int64) error {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil
	}
	amountMicros = nonNegativeInt64(amountMicros)
	wallet := domains.UserWallet{
		UserGuid:                  userGuid,
		BalanceAmountMicros:       amountMicros,
		PaidBalanceAmountMicros:   amountMicros,
		TotalRechargeAmountMicros: amountMicros,
		Currency:                  "CNY",
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&wallet).Error
}

func (s *UserWalletService) Get(userGuid string) (*domains.UserWallet, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	if err := s.DB().Transaction(func(tx *gorm.DB) error {
		return s.Ensure(tx, userGuid)
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

// GetBalanceAccount exposes the wallet through the admin balance view model.
// The legacy remain/used/total fields are calculated from wallet columns only.
func (s *UserWalletService) GetBalanceAccount(userGuid string) (*WalletBalanceAccount, error) {
	wallet, err := s.Get(userGuid)
	if err != nil {
		return nil, err
	}
	account := walletToBalanceAccount(wallet)
	return &account, nil
}

// ListBalanceAccounts is the admin balance list backed by user wallets.
func (s *UserWalletService) ListBalanceAccounts(query vos.PageQuery) (vos.PageResult, error) {
	query.Normalize()
	var wallets []domains.UserWallet
	var total int64
	db := s.DB().Model(&domains.UserWallet{})
	if query.Q != "" {
		keyword := "%" + strings.TrimSpace(query.Q) + "%"
		db = db.Where("user_guid LIKE ? OR currency LIKE ?", keyword, keyword)
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&wallets).Error; err != nil {
		return vos.PageResult{}, err
	}
	accounts := make([]WalletBalanceAccount, 0, len(wallets))
	for i := range wallets {
		accounts = append(accounts, walletToBalanceAccount(&wallets[i]))
	}
	return vos.PageResult{List: accounts, Total: total, Page: query.Page, Size: query.Size}, nil
}

// UpdateBalanceAccount calibrates an admin balance account directly on wallet data.
// It intentionally does not read or sync any removed legacy quota account model.
func (s *UserWalletService) UpdateBalanceAccount(input WalletBalanceAccount) (*WalletBalanceAccount, error) {
	input.UserGuid = strings.TrimSpace(input.UserGuid)
	if input.UserGuid == "" {
		return nil, errors.New("user guid is required")
	}
	var account WalletBalanceAccount
	err := s.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.Ensure(tx, input.UserGuid); err != nil {
			return err
		}
		wallet, err := s.lockWallet(tx, input.UserGuid)
		if err != nil {
			return err
		}
		applyBalanceAccountUpdate(wallet, input)
		if err := s.saveWallet(tx, wallet); err != nil {
			return err
		}
		account = walletToBalanceAccount(wallet)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *UserWalletService) ListRecords(userGuid string, query vos.PageQuery) (vos.PageResult, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return vos.PageResult{}, errors.New("user guid is required")
	}
	query.Normalize()
	var records []domains.UserWalletRecord
	var total int64
	db := s.RecordCrud.DB().Model(&domains.UserWalletRecord{}).Where("user_guid = ?", userGuid)
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		db = db.Where("type LIKE ? OR source LIKE ? OR title LIKE ? OR order_no LIKE ? OR remark LIKE ?", keyword, keyword, keyword, keyword, keyword)
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&records).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: records, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *UserWalletService) RecordIncome(tx *gorm.DB, input WalletRecordInput) error {
	input = normalizeWalletRecordInput(input)
	if input.UserGuid == "" {
		return nil
	}
	if input.AmountMicros <= 0 && input.AmountCents <= 0 {
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
	return s.createRecord(tx, wallet, input, domains.WalletRecordDirectionIncome, input.AmountMicros)
}

func (s *UserWalletService) RecordConsume(tx *gorm.DB, input WalletRecordInput) error {
	input = normalizeWalletRecordInput(input)
	if input.UserGuid == "" {
		return nil
	}
	if input.AmountMicros <= 0 && input.RequestCount <= 0 {
		return nil
	}
	if err := s.Ensure(tx, input.UserGuid); err != nil {
		return err
	}
	wallet, err := s.lockWallet(tx, input.UserGuid)
	if err != nil {
		return err
	}
	amountMicros := nonNegativeInt64(input.AmountMicros)
	requestCount := nonNegativeInt64(input.RequestCount)
	if wallet.BalanceAmountMicros < amountMicros {
		return errors.New("wallet balance is insufficient")
	}
	deductWalletAmount(wallet, amountMicros)
	wallet.TotalConsumedAmountMicros += amountMicros
	wallet.TotalRequestCount += requestCount
	if err := s.saveWallet(tx, wallet); err != nil {
		return err
	}
	input.Type = domains.WalletRecordTypeConsume
	input.Source = defaultString(input.Source, domains.WalletSourceRelay)
	input.RequestCount = requestCount
	return s.createRecord(tx, wallet, input, domains.WalletRecordDirectionOutcome, -amountMicros)
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
		"balance_amount_micros":            wallet.BalanceAmountMicros,
		"paid_balance_amount_micros":       wallet.PaidBalanceAmountMicros,
		"reward_balance_amount_micros":     wallet.RewardBalanceAmountMicros,
		"commission_balance_amount_micros": wallet.CommissionBalanceAmountMicros,
		"total_consumed_amount_micros":     wallet.TotalConsumedAmountMicros,
		"total_request_count":              wallet.TotalRequestCount,
		"total_recharge_amount_micros":     wallet.TotalRechargeAmountMicros,
		"total_subscription_amount_micros": wallet.TotalSubscriptionAmountMicros,
		"total_reward_amount_micros":       wallet.TotalRewardAmountMicros,
		"total_commission_amount_micros":   wallet.TotalCommissionAmountMicros,
		"total_recharge_amount_cents":      wallet.TotalRechargeAmountCents,
		"total_subscription_amount_cents":  wallet.TotalSubscriptionAmountCents,
		"currency":                         wallet.Currency,
	}).Error
}

func (s *UserWalletService) createRecord(tx *gorm.DB, wallet *domains.UserWallet, input WalletRecordInput, direction string, amountMicrosDelta int64) error {
	record := domains.UserWalletRecord{
		UserGuid:                           input.UserGuid,
		Type:                               input.Type,
		Direction:                          direction,
		Source:                             input.Source,
		Title:                              input.Title,
		RequestCountDelta:                  input.RequestCount,
		AmountMicrosDelta:                  amountMicrosDelta,
		BalanceAmountMicrosAfter:           wallet.BalanceAmountMicros,
		PaidBalanceAmountMicrosAfter:       wallet.PaidBalanceAmountMicros,
		RewardBalanceAmountMicrosAfter:     wallet.RewardBalanceAmountMicros,
		CommissionBalanceAmountMicrosAfter: wallet.CommissionBalanceAmountMicros,
		AmountCents:                        input.AmountCents,
		Currency:                           input.Currency,
		OrderNo:                            input.OrderNo,
		PaymentGuid:                        input.PaymentGuid,
		SubscriptionGuid:                   input.SubscriptionGuid,
		TokenID:                            input.TokenID,
		TokenGuid:                          input.TokenGuid,
		RelatedGuid:                        input.RelatedGuid,
		OccurredAt:                         input.OccurredAt,
		Remark:                             input.Remark,
		Meta:                               input.Meta,
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
	amountMicros := nonNegativeInt64(input.AmountMicros)
	wallet.BalanceAmountMicros += amountMicros
	switch input.Type {
	case domains.WalletRecordTypeSubscription:
		wallet.PaidBalanceAmountMicros += amountMicros
		wallet.TotalSubscriptionAmountMicros += amountMicros
		wallet.TotalSubscriptionAmountCents += nonNegativeInt64(input.AmountCents)
	case domains.WalletRecordTypeReward:
		wallet.RewardBalanceAmountMicros += amountMicros
		wallet.TotalRewardAmountMicros += amountMicros
	case domains.WalletRecordTypeCommission:
		wallet.CommissionBalanceAmountMicros += amountMicros
		wallet.TotalCommissionAmountMicros += amountMicros
	default:
		wallet.PaidBalanceAmountMicros += amountMicros
		wallet.TotalRechargeAmountMicros += amountMicros
		wallet.TotalRechargeAmountCents += nonNegativeInt64(input.AmountCents)
	}
}

func deductWalletAmount(wallet *domains.UserWallet, amountMicros int64) {
	remaining := nonNegativeInt64(amountMicros)
	take := int64Min(wallet.RewardBalanceAmountMicros, remaining)
	wallet.RewardBalanceAmountMicros -= take
	remaining -= take
	take = int64Min(wallet.CommissionBalanceAmountMicros, remaining)
	wallet.CommissionBalanceAmountMicros -= take
	remaining -= take
	if remaining > wallet.PaidBalanceAmountMicros {
		return
	}
	wallet.PaidBalanceAmountMicros -= remaining
	wallet.BalanceAmountMicros = wallet.PaidBalanceAmountMicros + wallet.RewardBalanceAmountMicros + wallet.CommissionBalanceAmountMicros
}

func normalizeWallet(wallet *domains.UserWallet) {
	wallet.UserGuid = strings.TrimSpace(wallet.UserGuid)
	wallet.TotalRequestCount = nonNegativeInt64(wallet.TotalRequestCount)
	wallet.TotalRechargeAmountCents = nonNegativeInt64(wallet.TotalRechargeAmountCents)
	wallet.TotalSubscriptionAmountCents = nonNegativeInt64(wallet.TotalSubscriptionAmountCents)
	normalizeWalletAmounts(wallet)
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
	input.AmountCents = nonNegativeInt64(input.AmountCents)
	input.AmountMicros = nonNegativeInt64(input.AmountMicros)
	if input.AmountMicros <= 0 && input.AmountCents > 0 {
		input.AmountMicros = AmountCentsToMicros(input.AmountCents)
	}
	if input.Type == "" {
		input.Type = domains.WalletRecordTypeRecharge
	}
	if input.Source == "" {
		input.Source = domains.WalletSourceManual
	}
	return input
}

func normalizeWalletAmounts(wallet *domains.UserWallet) {
	wallet.PaidBalanceAmountMicros = nonNegativeInt64(wallet.PaidBalanceAmountMicros)
	wallet.RewardBalanceAmountMicros = nonNegativeInt64(wallet.RewardBalanceAmountMicros)
	wallet.CommissionBalanceAmountMicros = nonNegativeInt64(wallet.CommissionBalanceAmountMicros)
	wallet.TotalConsumedAmountMicros = nonNegativeInt64(wallet.TotalConsumedAmountMicros)
	wallet.TotalRechargeAmountMicros = nonNegativeInt64(wallet.TotalRechargeAmountMicros)
	wallet.TotalSubscriptionAmountMicros = nonNegativeInt64(wallet.TotalSubscriptionAmountMicros)
	wallet.TotalRewardAmountMicros = nonNegativeInt64(wallet.TotalRewardAmountMicros)
	wallet.TotalCommissionAmountMicros = nonNegativeInt64(wallet.TotalCommissionAmountMicros)
	if wallet.TotalRechargeAmountMicros <= 0 && wallet.TotalRechargeAmountCents > 0 {
		wallet.TotalRechargeAmountMicros = AmountCentsToMicros(wallet.TotalRechargeAmountCents)
	}
	if wallet.TotalSubscriptionAmountMicros <= 0 && wallet.TotalSubscriptionAmountCents > 0 {
		wallet.TotalSubscriptionAmountMicros = AmountCentsToMicros(wallet.TotalSubscriptionAmountCents)
	}
	wallet.BalanceAmountMicros = wallet.PaidBalanceAmountMicros + wallet.RewardBalanceAmountMicros + wallet.CommissionBalanceAmountMicros
}

func walletToBalanceAccount(wallet *domains.UserWallet) WalletBalanceAccount {
	normalizeWallet(wallet)
	totalAmountMicros := walletTotalIncomeAmountMicros(wallet)
	if totalAmountMicros <= 0 && wallet.BalanceAmountMicros+wallet.TotalConsumedAmountMicros > 0 {
		totalAmountMicros = wallet.BalanceAmountMicros + wallet.TotalConsumedAmountMicros
	}
	return WalletBalanceAccount{
		BaseDataEntity:                wallet.BaseDataEntity,
		UserGuid:                      wallet.UserGuid,
		BalanceAmountMicros:           wallet.BalanceAmountMicros,
		PaidBalanceAmountMicros:       wallet.PaidBalanceAmountMicros,
		RewardBalanceAmountMicros:     wallet.RewardBalanceAmountMicros,
		CommissionBalanceAmountMicros: wallet.CommissionBalanceAmountMicros,
		TotalConsumedAmountMicros:     wallet.TotalConsumedAmountMicros,
		TotalRequestCount:             wallet.TotalRequestCount,
		TotalRechargeAmountMicros:     wallet.TotalRechargeAmountMicros,
		TotalSubscriptionAmountMicros: wallet.TotalSubscriptionAmountMicros,
		TotalRewardAmountMicros:       wallet.TotalRewardAmountMicros,
		TotalCommissionAmountMicros:   wallet.TotalCommissionAmountMicros,
		TotalRechargeAmountCents:      wallet.TotalRechargeAmountCents,
		TotalSubscriptionAmountCents:  wallet.TotalSubscriptionAmountCents,
		Currency:                      wallet.Currency,
		RemainAmountMicros:            wallet.BalanceAmountMicros,
		UsedAmountMicros:              wallet.TotalConsumedAmountMicros,
		TotalAmountMicros:             totalAmountMicros,
		Group:                         "default",
		AllowedGroups:                 "",
	}
}

func applyBalanceAccountUpdate(wallet *domains.UserWallet, input WalletBalanceAccount) {
	paidBalance := input.PaidBalanceAmountMicros
	rewardBalance := input.RewardBalanceAmountMicros
	commissionBalance := input.CommissionBalanceAmountMicros
	if paidBalance <= 0 && rewardBalance <= 0 && commissionBalance <= 0 {
		paidBalance = firstPositiveInt64(input.BalanceAmountMicros, input.RemainAmountMicros)
	}
	wallet.PaidBalanceAmountMicros = paidBalance
	wallet.RewardBalanceAmountMicros = rewardBalance
	wallet.CommissionBalanceAmountMicros = commissionBalance
	wallet.TotalConsumedAmountMicros = firstPositiveInt64(input.TotalConsumedAmountMicros, input.UsedAmountMicros)
	wallet.TotalRequestCount = input.TotalRequestCount
	wallet.TotalRechargeAmountMicros = firstPositiveInt64(input.TotalRechargeAmountMicros, input.TotalAmountMicros)
	wallet.TotalSubscriptionAmountMicros = input.TotalSubscriptionAmountMicros
	wallet.TotalRewardAmountMicros = input.TotalRewardAmountMicros
	wallet.TotalCommissionAmountMicros = input.TotalCommissionAmountMicros
	wallet.TotalRechargeAmountCents = input.TotalRechargeAmountCents
	wallet.TotalSubscriptionAmountCents = input.TotalSubscriptionAmountCents
	wallet.Currency = input.Currency
	normalizeWallet(wallet)
}

func walletTotalIncomeAmountMicros(wallet *domains.UserWallet) int64 {
	return nonNegativeInt64(wallet.TotalRechargeAmountMicros) +
		nonNegativeInt64(wallet.TotalSubscriptionAmountMicros) +
		nonNegativeInt64(wallet.TotalRewardAmountMicros) +
		nonNegativeInt64(wallet.TotalCommissionAmountMicros)
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
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
