package services

import (
	"errors"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenService struct{}

var TokenServiceApp = TokenService{}

type TokenUsage struct {
	ID               uint   `json:"id"`
	Guid             string `json:"guid"`
	Name             string `json:"name"`
	Status           int    `json:"status"`
	Group            string `json:"group"`
	RemainQuota      int64  `json:"remainQuota"`
	UnlimitedQuota   bool   `json:"unlimitedQuota"`
	UsedQuota        int64  `json:"usedQuota"`
	AccessedTime     int64  `json:"accessedTime"`
	TotalRequests    int64  `json:"totalRequests"`
	SuccessRequests  int64  `json:"successRequests"`
	ErrorRequests    int64  `json:"errorRequests"`
	LogQuota         int64  `json:"logQuota"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
}

func (s TokenService) Create(token *domains.ApiToken) error {
	key, err := randomHex(24)
	if err != nil {
		return err
	}
	token.Key = "sk-" + key
	token.Status = constants.StatusEnabled
	token.Group = normalizeGroup(token.Group)
	if token.ExpiredTime == 0 {
		token.ExpiredTime = -1
	}
	return global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		if err := UserQuotaServiceApp.Ensure(tx, token.UserGuid); err != nil {
			return err
		}
		if err := UserQuotaServiceApp.CheckGroup(token.UserGuid, token.Group); err != nil {
			return err
		}
		return tx.Create(token).Error
	})
}

func (s TokenService) Update(token *domains.ApiToken) error {
	if token.Group == "" {
		token.Group = constants.DefaultGroup
	}
	if err := UserQuotaServiceApp.CheckGroup(token.UserGuid, token.Group); err != nil {
		return err
	}
	return global.NAV_DB.Save(token).Error
}

func (s TokenService) Delete(id uint, userGuid string) error {
	db := global.NAV_DB.Where("id = ?", id)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	return db.Delete(&domains.ApiToken{}).Error
}

func (s TokenService) GetByID(id uint, userGuid string) (*domains.ApiToken, error) {
	var token domains.ApiToken
	db := global.NAV_DB.Where("id = ?", id)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if err := db.First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (s TokenService) List(userGuid string) ([]domains.ApiToken, error) {
	var tokens []domains.ApiToken
	db := global.NAV_DB.Order("id desc")
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	err := db.Find(&tokens).Error
	return tokens, err
}

func (s TokenService) Validate(key string, clientIP string) (*domains.ApiToken, error) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "Bearer "))
	if key == "" {
		return nil, errors.New("token is required")
	}
	var token domains.ApiToken
	if err := global.NAV_DB.Where("key = ?", key).First(&token).Error; err != nil {
		return nil, err
	}
	if token.Status != constants.StatusEnabled {
		return nil, errors.New("token is disabled")
	}
	now := time.Now().Unix()
	if token.ExpiredTime > 0 && token.ExpiredTime < now {
		return nil, errors.New("token is expired")
	}
	if !token.UnlimitedQuota && token.RemainQuota <= 0 {
		return nil, errors.New("token quota is exhausted")
	}
	if token.AllowIPs != "" && !containsString(splitCSV(token.AllowIPs), clientIP) {
		return nil, errors.New("client ip is not allowed")
	}
	token.AccessedTime = now
	_ = global.NAV_DB.Model(&domains.ApiToken{}).Where("id = ?", token.Id).Update("accessed_time", now).Error
	return &token, nil
}

func (s TokenService) CheckModel(token *domains.ApiToken, modelName string) error {
	if token == nil {
		return errors.New("token is required")
	}
	if !token.ModelLimitsEnabled {
		return nil
	}
	if containsString(splitCSV(token.ModelLimits), modelName) {
		return nil
	}
	return errors.New("model is not allowed by token")
}

func (s TokenService) Consume(tx *gorm.DB, id uint, quota int64) error {
	if quota <= 0 {
		return nil
	}
	var token domains.ApiToken
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&token, id).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"used_quota": gorm.Expr("used_quota + ?", quota),
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < quota {
			return errors.New("token quota is exhausted")
		}
		updates["remain_quota"] = gorm.Expr("remain_quota - ?", quota)
	}
	return tx.Model(&domains.ApiToken{}).Where("id = ?", id).Updates(updates).Error
}

// Refund reverses a previous quota reservation/charge. It is intentionally
// bounded at zero for used_quota so repeated cleanup cannot drive counters
// negative after retries or client disconnects.
func (s TokenService) Refund(tx *gorm.DB, id uint, quota int64) error {
	if quota <= 0 {
		return nil
	}
	var token domains.ApiToken
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&token, id).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"used_quota": gorm.Expr("CASE WHEN used_quota >= ? THEN used_quota - ? ELSE 0 END", quota, quota),
	}
	if !token.UnlimitedQuota {
		updates["remain_quota"] = gorm.Expr("remain_quota + ?", quota)
	}
	return tx.Model(&domains.ApiToken{}).Where("id = ?", id).Updates(updates).Error
}

func (s TokenService) AddQuota(tx *gorm.DB, id uint, userGuid string, quota int64) error {
	if quota <= 0 {
		return errors.New("quota must be greater than zero")
	}
	var token domains.ApiToken
	db := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if err := db.First(&token).Error; err != nil {
		return err
	}
	if err := UserQuotaServiceApp.AddQuota(tx, token.UserGuid, quota); err != nil {
		return err
	}
	return tx.Model(&domains.ApiToken{}).Where("id = ?", token.Id).
		UpdateColumn("remain_quota", gorm.Expr("remain_quota + ?", quota)).Error
}

func (s TokenService) Usage(userGuid string) ([]TokenUsage, error) {
	tokens, err := s.List(userGuid)
	if err != nil {
		return nil, err
	}
	tokenGuids := make([]string, 0, len(tokens))
	for _, token := range tokens {
		tokenGuids = append(tokenGuids, token.Guid)
	}
	stats := map[string]TokenUsage{}
	if len(tokenGuids) > 0 {
		var rows []struct {
			TokenGuid        string
			TotalRequests    int64
			SuccessRequests  int64
			Quota            int64
			PromptTokens     int64
			CompletionTokens int64
		}
		if err := global.NAV_DB.Model(&domains.UsageLog{}).
			Select("token_guid, COUNT(*) AS total_requests, COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END),0) AS success_requests, COALESCE(SUM(quota),0) AS quota, COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens").
			Where("token_guid IN ?", tokenGuids).
			Group("token_guid").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			stats[row.TokenGuid] = TokenUsage{
				TotalRequests:    row.TotalRequests,
				SuccessRequests:  row.SuccessRequests,
				ErrorRequests:    row.TotalRequests - row.SuccessRequests,
				LogQuota:         row.Quota,
				PromptTokens:     row.PromptTokens,
				CompletionTokens: row.CompletionTokens,
			}
		}
	}
	out := make([]TokenUsage, 0, len(tokens))
	for _, token := range tokens {
		usage := stats[token.Guid]
		usage.ID = token.Id
		usage.Guid = token.Guid
		usage.Name = token.Name
		usage.Status = token.Status
		usage.Group = token.Group
		usage.RemainQuota = token.RemainQuota
		usage.UnlimitedQuota = token.UnlimitedQuota
		usage.UsedQuota = token.UsedQuota
		usage.AccessedTime = token.AccessedTime
		out = append(out, usage)
	}
	return out, nil
}

func (s TokenService) Mask(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return key[:2] + "****"
	}
	return key[:6] + "********" + key[len(key)-4:]
}
