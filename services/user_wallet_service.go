package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

type WalletRecordQuery struct {
	vos.PageQuery
	Type      string `form:"type" json:"type"`
	Source    string `form:"source" json:"source"`
	Direction string `form:"direction" json:"direction"`
	StartTime int64  `form:"startTime" json:"startTime"`
	EndTime   int64  `form:"endTime" json:"endTime"`
}

func (q *WalletRecordQuery) Normalize() {
	q.PageQuery.Normalize()
	q.Type = strings.ToLower(strings.TrimSpace(q.Type))
	q.Source = strings.ToLower(strings.TrimSpace(q.Source))
	q.Direction = strings.ToLower(strings.TrimSpace(q.Direction))
	q.StartTime = normalizeUnixSeconds(q.StartTime)
	q.EndTime = normalizeUnixSeconds(q.EndTime)
	if q.StartTime > 0 && q.EndTime > 0 && q.EndTime < q.StartTime {
		q.EndTime = 0
	}
}

type WalletActivityQuery struct {
	StartTime int64 `form:"startTime" json:"startTime"`
	EndTime   int64 `form:"endTime" json:"endTime"`
}

func (q *WalletActivityQuery) Normalize() {
	q.StartTime = normalizeUnixSeconds(q.StartTime)
	q.EndTime = normalizeUnixSeconds(q.EndTime)
	if q.StartTime > 0 && q.EndTime > 0 && q.EndTime < q.StartTime {
		q.EndTime = 0
	}
}

type WalletActivityItem struct {
	ID                                 string `json:"id"`
	Category                           string `json:"category"`
	Aggregate                          bool   `json:"aggregate"`
	Title                              string `json:"title"`
	Source                             string `json:"source"`
	Timestamp                          int64  `json:"timestamp"`
	EndTimestamp                       int64  `json:"endTimestamp,omitempty"`
	AmountMicrosDelta                  int64  `json:"amountMicrosDelta"`
	BalanceAmountMicrosAfter           int64  `json:"balanceAmountMicrosAfter"`
	Currency                           string `json:"currency"`
	Count                              int64  `json:"count"`
	RequestCount                       int64  `json:"requestCount"`
	Guid                               string `json:"guid,omitempty"`
	RecordID                           uint   `json:"recordId,omitempty"`
	RelatedGuid                        string `json:"relatedGuid,omitempty"`
	Remark                             string `json:"remark,omitempty"`
	PaidBalanceAmountMicrosAfter       int64  `json:"paidBalanceAmountMicrosAfter,omitempty"`
	RewardBalanceAmountMicrosAfter     int64  `json:"rewardBalanceAmountMicrosAfter,omitempty"`
	CommissionBalanceAmountMicrosAfter int64  `json:"commissionBalanceAmountMicrosAfter,omitempty"`
}

type walletDeduction struct {
	Paid       int64 `json:"paid"`
	Reward     int64 `json:"reward"`
	Commission int64 `json:"commission"`
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

func (s *UserWalletService) ListRecords(userGuid string, query WalletRecordQuery) (vos.PageResult, error) {
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
	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.Source != "" {
		db = db.Where("source = ?", query.Source)
	}
	if query.Direction != "" {
		db = db.Where("direction = ?", query.Direction)
	}
	if query.StartTime > 0 {
		db = db.Where("occurred_at >= ?", query.StartTime)
	}
	if query.EndTime > 0 {
		db = db.Where("occurred_at <= ?", query.EndTime)
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&records).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: records, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *UserWalletService) ListActivities(userGuid string, query WalletActivityQuery) ([]WalletActivityItem, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	query.Normalize()
	db := s.RecordCrud.DB().Model(&domains.UserWalletRecord{}).Where("user_guid = ?", userGuid)
	db = applyWalletActivityTimeRange(db, query)

	var records []domains.UserWalletRecord
	if err := db.
		Where("amount_micros_delta <> 0").
		Where("NOT " + walletAPIConsumeSQL()).
		Order("occurred_at desc, id desc").
		Find(&records).Error; err != nil {
		return nil, err
	}

	activities := make([]WalletActivityItem, 0, len(records))
	for i := range records {
		activities = append(activities, walletRecordActivity(records[i]))
	}

	apiActivities, err := s.listAPIConsumeActivities(userGuid, query)
	if err != nil {
		return nil, err
	}
	activities = append(activities, apiActivities...)
	sort.SliceStable(activities, func(i, j int) bool {
		if activities[i].Timestamp == activities[j].Timestamp {
			return activities[i].ID > activities[j].ID
		}
		return activities[i].Timestamp > activities[j].Timestamp
	})
	return activities, nil
}

type walletAPIConsumeAggregate struct {
	HourStart         int64
	Count             int64
	RequestCount      int64
	AmountMicrosDelta int64
}

func (s *UserWalletService) listAPIConsumeActivities(userGuid string, query WalletActivityQuery) ([]WalletActivityItem, error) {
	hourExpr := walletHourBucketExpr(s.RecordCrud.DB())
	db := s.RecordCrud.DB().
		Model(&domains.UserWalletRecord{}).
		Select(fmt.Sprintf("%s AS hour_start, COUNT(*) AS count, COALESCE(SUM(request_count_delta), 0) AS request_count, COALESCE(SUM(amount_micros_delta), 0) AS amount_micros_delta", hourExpr)).
		Where("user_guid = ?", userGuid).
		Where(walletAPIConsumeSQL()).
		Group(hourExpr)
	db = applyWalletActivityTimeRange(db, query)

	var rows []walletAPIConsumeAggregate
	if err := db.Scan(&rows).Error; err != nil {
		return nil, err
	}
	activities := make([]WalletActivityItem, 0, len(rows))
	for _, row := range rows {
		if row.HourStart <= 0 || row.Count <= 0 {
			continue
		}
		latest, err := s.latestAPIConsumeRecord(userGuid, row.HourStart)
		if err != nil {
			return nil, err
		}
		activities = append(activities, WalletActivityItem{
			ID:                       fmt.Sprintf("api-consume-%d", row.HourStart),
			Category:                 domains.WalletRecordTypeConsume,
			Aggregate:                true,
			Title:                    "API 消费汇总",
			Source:                   domains.WalletSourceRelay,
			Timestamp:                row.HourStart,
			EndTimestamp:             row.HourStart + 3599,
			AmountMicrosDelta:        row.AmountMicrosDelta,
			BalanceAmountMicrosAfter: latest.BalanceAmountMicrosAfter,
			Currency:                 defaultString(latest.Currency, "CNY"),
			Count:                    row.Count,
			RequestCount:             firstPositiveInt64(row.RequestCount, row.Count),
		})
	}
	return activities, nil
}

func (s *UserWalletService) latestAPIConsumeRecord(userGuid string, hourStart int64) (domains.UserWalletRecord, error) {
	var record domains.UserWalletRecord
	err := s.RecordCrud.DB().
		Where("user_guid = ?", userGuid).
		Where(walletAPIConsumeSQL()).
		Where("occurred_at >= ? AND occurred_at <= ?", hourStart, hourStart+3599).
		Order("occurred_at desc, id desc").
		First(&record).Error
	return record, err
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

func (s *UserWalletService) ReserveConsume(tx *gorm.DB, input WalletRecordInput) (*domains.UserWalletRecord, error) {
	input = normalizeWalletRecordInput(input)
	if input.UserGuid == "" {
		return nil, nil
	}
	if input.AmountMicros <= 0 {
		return nil, nil
	}
	if err := s.Ensure(tx, input.UserGuid); err != nil {
		return nil, err
	}
	wallet, err := s.lockWallet(tx, input.UserGuid)
	if err != nil {
		return nil, err
	}
	amountMicros := nonNegativeInt64(input.AmountMicros)
	requestCount := nonNegativeInt64(input.RequestCount)
	if wallet.BalanceAmountMicros < amountMicros {
		return nil, errors.New("wallet balance is insufficient")
	}
	deduction := deductWalletAmountDetail(wallet, amountMicros)
	wallet.TotalConsumedAmountMicros += amountMicros
	wallet.TotalRequestCount += requestCount
	if err := s.saveWallet(tx, wallet); err != nil {
		return nil, err
	}
	input.Type = domains.WalletRecordTypeConsume
	input.Source = defaultString(input.Source, domains.WalletSourceRelay)
	input.Title = defaultString(input.Title, "API 消费预授权")
	input.RequestCount = requestCount
	input.Meta = mergeWalletRecordMeta(input.Meta, map[string]any{
		"reserved":             true,
		"reservedAmountMicros": amountMicros,
		"reservationDeduction": deduction,
	})
	return s.createRecordWithResult(tx, wallet, input, domains.WalletRecordDirectionOutcome, -amountMicros)
}

func (s *UserWalletService) FinalizeReservedConsume(tx *gorm.DB, recordID uint, input WalletRecordInput) error {
	if recordID == 0 {
		return s.RecordConsume(tx, input)
	}
	input = normalizeWalletRecordInput(input)
	var record domains.UserWalletRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, recordID).Error; err != nil {
		return err
	}
	if record.UserGuid == "" {
		return errors.New("wallet reservation is invalid")
	}
	wallet, err := s.lockWallet(tx, record.UserGuid)
	if err != nil {
		return err
	}
	reservedAmount := nonNegativeInt64(-record.AmountMicrosDelta)
	finalAmount := nonNegativeInt64(input.AmountMicros)
	delta := finalAmount - reservedAmount
	if delta > 0 {
		if wallet.BalanceAmountMicros < delta {
			return errors.New("wallet balance is insufficient")
		}
		deductWalletAmount(wallet, delta)
	} else if delta < 0 {
		refundReservedWalletAmount(wallet, -delta, walletRecordReservationDeduction(record.Meta))
	}
	wallet.TotalConsumedAmountMicros = nonNegativeInt64(wallet.TotalConsumedAmountMicros + delta)
	requestCount := nonNegativeInt64(input.RequestCount)
	if requestCount <= 0 {
		requestCount = nonNegativeInt64(record.RequestCountDelta)
	}
	wallet.TotalRequestCount = nonNegativeInt64(wallet.TotalRequestCount - nonNegativeInt64(record.RequestCountDelta) + requestCount)
	if err := s.saveWallet(tx, wallet); err != nil {
		return err
	}
	return s.updateReservedRecord(tx, record.Id, wallet, input, requestCount, -finalAmount)
}

func (s *UserWalletService) CancelReservedConsume(tx *gorm.DB, recordID uint, reason string) error {
	if recordID == 0 {
		return nil
	}
	var record domains.UserWalletRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, recordID).Error; err != nil {
		return err
	}
	wallet, err := s.lockWallet(tx, record.UserGuid)
	if err != nil {
		return err
	}
	reservedAmount := nonNegativeInt64(-record.AmountMicrosDelta)
	if reservedAmount > 0 {
		refundReservedWalletAmount(wallet, reservedAmount, walletRecordReservationDeduction(record.Meta))
	}
	wallet.TotalConsumedAmountMicros = nonNegativeInt64(wallet.TotalConsumedAmountMicros - reservedAmount)
	wallet.TotalRequestCount = nonNegativeInt64(wallet.TotalRequestCount - nonNegativeInt64(record.RequestCountDelta))
	if err := s.saveWallet(tx, wallet); err != nil {
		return err
	}
	if len(reason) > 255 {
		reason = reason[:255]
	}
	return tx.Model(&domains.UserWalletRecord{}).Where("id = ?", record.Id).Updates(map[string]any{
		"title":                                  "API 消费取消",
		"request_count_delta":                    0,
		"amount_micros_delta":                    0,
		"balance_amount_micros_after":            wallet.BalanceAmountMicros,
		"paid_balance_amount_micros_after":       wallet.PaidBalanceAmountMicros,
		"reward_balance_amount_micros_after":     wallet.RewardBalanceAmountMicros,
		"commission_balance_amount_micros_after": wallet.CommissionBalanceAmountMicros,
		"remark":                                 reason,
	}).Error
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
	_, err := s.createRecordWithResult(tx, wallet, input, direction, amountMicrosDelta)
	return err
}

func (s *UserWalletService) createRecordWithResult(tx *gorm.DB, wallet *domains.UserWallet, input WalletRecordInput, direction string, amountMicrosDelta int64) (*domains.UserWalletRecord, error) {
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
		return nil, err
	}
	if err := tx.Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *UserWalletService) updateReservedRecord(tx *gorm.DB, recordID uint, wallet *domains.UserWallet, input WalletRecordInput, requestCount int64, amountMicrosDelta int64) error {
	return tx.Model(&domains.UserWalletRecord{}).Where("id = ?", recordID).Updates(map[string]any{
		"type":                                   domains.WalletRecordTypeConsume,
		"direction":                              domains.WalletRecordDirectionOutcome,
		"source":                                 defaultString(input.Source, domains.WalletSourceRelay),
		"title":                                  defaultString(input.Title, "API 消费"),
		"request_count_delta":                    requestCount,
		"amount_micros_delta":                    amountMicrosDelta,
		"balance_amount_micros_after":            wallet.BalanceAmountMicros,
		"paid_balance_amount_micros_after":       wallet.PaidBalanceAmountMicros,
		"reward_balance_amount_micros_after":     wallet.RewardBalanceAmountMicros,
		"commission_balance_amount_micros_after": wallet.CommissionBalanceAmountMicros,
		"amount_cents":                           input.AmountCents,
		"currency":                               input.Currency,
		"order_no":                               input.OrderNo,
		"payment_guid":                           input.PaymentGuid,
		"subscription_guid":                      input.SubscriptionGuid,
		"token_id":                               input.TokenID,
		"token_guid":                             input.TokenGuid,
		"related_guid":                           input.RelatedGuid,
		"remark":                                 input.Remark,
		"meta":                                   input.Meta,
	}).Error
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
	_ = deductWalletAmountDetail(wallet, amountMicros)
}

func deductWalletAmountDetail(wallet *domains.UserWallet, amountMicros int64) walletDeduction {
	remaining := nonNegativeInt64(amountMicros)
	deduction := walletDeduction{}
	take := int64Min(wallet.RewardBalanceAmountMicros, remaining)
	wallet.RewardBalanceAmountMicros -= take
	deduction.Reward = take
	remaining -= take
	take = int64Min(wallet.CommissionBalanceAmountMicros, remaining)
	wallet.CommissionBalanceAmountMicros -= take
	deduction.Commission = take
	remaining -= take
	if remaining > wallet.PaidBalanceAmountMicros {
		return deduction
	}
	wallet.PaidBalanceAmountMicros -= remaining
	deduction.Paid = remaining
	wallet.BalanceAmountMicros = wallet.PaidBalanceAmountMicros + wallet.RewardBalanceAmountMicros + wallet.CommissionBalanceAmountMicros
	return deduction
}

func refundReservedWalletAmount(wallet *domains.UserWallet, amountMicros int64, deduction walletDeduction) {
	remaining := nonNegativeInt64(amountMicros)
	take := int64Min(deduction.Paid, remaining)
	wallet.PaidBalanceAmountMicros += take
	remaining -= take
	take = int64Min(deduction.Commission, remaining)
	wallet.CommissionBalanceAmountMicros += take
	remaining -= take
	take = int64Min(deduction.Reward, remaining)
	wallet.RewardBalanceAmountMicros += take
	remaining -= take
	if remaining > 0 {
		wallet.PaidBalanceAmountMicros += remaining
	}
	wallet.BalanceAmountMicros = wallet.PaidBalanceAmountMicros + wallet.RewardBalanceAmountMicros + wallet.CommissionBalanceAmountMicros
}

func walletRecordReservationDeduction(raw string) walletDeduction {
	var payload struct {
		ReservationDeduction walletDeduction `json:"reservationDeduction"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return walletDeduction{}
	}
	return payload.ReservationDeduction
}

func mergeWalletRecordMeta(raw string, extra map[string]any) string {
	values := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &values)
	}
	for key, value := range extra {
		values[key] = value
	}
	data, err := json.Marshal(values)
	if err != nil {
		return raw
	}
	return string(data)
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

func walletRecordActivity(record domains.UserWalletRecord) WalletActivityItem {
	timestamp := normalizeUnixSeconds(record.OccurredAt)
	if timestamp <= 0 {
		timestamp = normalizeUnixSeconds(record.CreateTime)
	}
	return WalletActivityItem{
		ID:                                 walletRecordActivityID(record),
		Category:                           walletRecordCategory(record),
		Aggregate:                          false,
		Title:                              defaultString(record.Title, walletRecordTypeText(record.Type)),
		Source:                             record.Source,
		Timestamp:                          timestamp,
		AmountMicrosDelta:                  record.AmountMicrosDelta,
		BalanceAmountMicrosAfter:           record.BalanceAmountMicrosAfter,
		Currency:                           defaultString(record.Currency, "CNY"),
		Count:                              1,
		RequestCount:                       nonNegativeInt64(record.RequestCountDelta),
		Guid:                               record.Guid,
		RecordID:                           record.Id,
		RelatedGuid:                        record.RelatedGuid,
		Remark:                             record.Remark,
		PaidBalanceAmountMicrosAfter:       record.PaidBalanceAmountMicrosAfter,
		RewardBalanceAmountMicrosAfter:     record.RewardBalanceAmountMicrosAfter,
		CommissionBalanceAmountMicrosAfter: record.CommissionBalanceAmountMicrosAfter,
	}
}

func walletRecordActivityID(record domains.UserWalletRecord) string {
	if strings.TrimSpace(record.Guid) != "" {
		return record.Guid
	}
	if record.Id > 0 {
		return fmt.Sprintf("wallet-record-%d", record.Id)
	}
	return fmt.Sprintf("wallet-record-%d-%s-%d", normalizeUnixSeconds(record.OccurredAt), record.Type, record.AmountMicrosDelta)
}

func walletRecordCategory(record domains.UserWalletRecord) string {
	recordType := strings.ToLower(strings.TrimSpace(record.Type))
	source := strings.ToLower(strings.TrimSpace(record.Source))
	if source == domains.WalletSourceRedemption {
		return "redemption"
	}
	if recordType == domains.WalletRecordTypeSubscription || source == domains.WalletSourceSubscription {
		return domains.WalletRecordTypeSubscription
	}
	if recordType == domains.WalletRecordTypeCommission || source == domains.WalletSourceInvitation {
		return domains.WalletRecordTypeCommission
	}
	if recordType == domains.WalletRecordTypeReward || source == domains.WalletSourceCheckin {
		return domains.WalletRecordTypeReward
	}
	if recordType == domains.WalletRecordTypeConsume || source == domains.WalletSourceRelay {
		return domains.WalletRecordTypeConsume
	}
	if recordType == domains.WalletRecordTypeRecharge || source == domains.WalletSourcePayment {
		return domains.WalletRecordTypeRecharge
	}
	return "other"
}

func walletRecordTypeText(recordType string) string {
	switch strings.ToLower(strings.TrimSpace(recordType)) {
	case domains.WalletRecordTypeRecharge:
		return "充值"
	case domains.WalletRecordTypeSubscription:
		return "订阅"
	case domains.WalletRecordTypeConsume:
		return "消费"
	case domains.WalletRecordTypeReward:
		return "奖励"
	case domains.WalletRecordTypeCommission:
		return "分佣"
	default:
		return "资金流水"
	}
}

func applyWalletActivityTimeRange(db *gorm.DB, query WalletActivityQuery) *gorm.DB {
	if query.StartTime > 0 {
		db = db.Where("occurred_at >= ?", query.StartTime)
	}
	if query.EndTime > 0 {
		db = db.Where("occurred_at <= ?", query.EndTime)
	}
	return db
}

func walletAPIConsumeSQL() string {
	return "(amount_micros_delta < 0 AND (type = 'consume' OR source = 'relay' OR LOWER(title) LIKE '%api%'))"
}

func walletHourBucketExpr(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "mysql", "postgres":
		return "(FLOOR(occurred_at / 3600) * 3600)"
	default:
		return "((occurred_at / 3600) * 3600)"
	}
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

func normalizeUnixSeconds(value int64) int64 {
	if value > 1_000_000_000_000 {
		return value / 1000
	}
	return value
}
