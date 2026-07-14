package services

import (
	"errors"
	"strings"
	"time"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"navapi-go/constants"
	"navapi-go/domains"
)

type TokenService struct {
	commonServices.CrudService[domains.ApiToken]
}

var TokenServiceApp = new(TokenService)

func (s *TokenService) WithDB(db *gorm.DB) *TokenService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type TokenUsage struct {
	ID                  uint   `json:"id"`
	Guid                string `json:"guid"`
	Name                string `json:"name"`
	Status              int    `json:"status"`
	Group               string `json:"group"`
	UnlimitedBalance    bool   `json:"unlimitedBalance"`
	BalanceAmountMicros int64  `json:"balanceAmountMicros"`
	UsedAmountMicros    int64  `json:"usedAmountMicros"`
	AccessedTime        int64  `json:"accessedTime"`
	TotalRequests       int64  `json:"totalRequests"`
	SuccessRequests     int64  `json:"successRequests"`
	ErrorRequests       int64  `json:"errorRequests"`
	LogQuota            int64  `json:"logQuota"`
	PromptTokens        int64  `json:"promptTokens"`
	CompletionTokens    int64  `json:"completionTokens"`
}

func (s *TokenService) Create(token *domains.ApiToken) error {
	token.UserGuid = strings.TrimSpace(token.UserGuid)
	if token.UserGuid == "" {
		return errors.New("user guid is required")
	}
	key, err := randomHex(24)
	if err != nil {
		return err
	}
	token.Key = "sk-" + key
	token.Status = constants.StatusEnabled
	token.Group = normalizeGroup(token.Group)
	if err := ModelServiceApp.ValidateGroup(token.Group); err != nil {
		return err
	}
	if token.ExpiredTime == 0 {
		token.ExpiredTime = -1
	}
	normalizeTokenAmounts(token)
	return s.DB().Transaction(func(tx *gorm.DB) error {
		if err := token.BeforeCreate(nil); err != nil {
			return err
		}
		tokenCrud := s.CrudService.WithDB(tx)
		return tokenCrud.Create(*token)
	})
}

func (s *TokenService) Update(token *domains.ApiToken) error {
	token.Group = normalizeGroup(token.Group)
	if err := ModelServiceApp.ValidateGroup(token.Group); err != nil {
		return err
	}
	existing, err := s.getExisting(token.Id, token.Guid, token.UserGuid)
	if err != nil {
		return err
	}
	token.Id = existing.Id
	token.Guid = existing.Guid
	token.CreateTime = existing.CreateTime
	token.Creater = existing.Creater
	token.Updater = existing.Updater
	token.UpdateTime = time.Now().UnixMilli()
	normalizeTokenAmounts(token)
	if err := s.DB().Save(token).Error; err != nil {
		return err
	}
	return reloadByGuidWithCrud(&s.CrudService, token)
}

func (s *TokenService) Delete(id uint, userGuid string) error {
	token, err := s.GetByID(id, userGuid)
	if err != nil {
		return err
	}
	return s.DeleteByGuid(token.Guid)
}

func (s *TokenService) DeleteByGUID(guid string, userGuid string) error {
	token, err := s.GetByGUID(guid, userGuid)
	if err != nil {
		return err
	}
	return s.DeleteByGuid(token.Guid)
}

func (s *TokenService) GetByID(id uint, userGuid string) (*domains.ApiToken, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	token, err := s.GetById(id)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("token not found")
	}
	if userGuid != "" && token.UserGuid != userGuid {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (s *TokenService) GetByGUID(guid string, userGuid string) (*domains.ApiToken, error) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil, errors.New("guid is required")
	}
	token, err := s.GetByGuid(guid)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("token not found")
	}
	if userGuid != "" && token.UserGuid != userGuid {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (s *TokenService) getExisting(id uint, guid string, userGuid string) (*domains.ApiToken, error) {
	if strings.TrimSpace(guid) != "" {
		return s.GetByGUID(guid, userGuid)
	}
	return s.GetByID(id, userGuid)
}

func (s *TokenService) List(userGuid string) ([]domains.ApiToken, error) {
	var tokens []domains.ApiToken
	db := s.DB().Order("id desc")
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if err := db.Find(&tokens).Error; err != nil {
		return nil, err
	}
	s.fillTokenUsers(tokens)
	return tokens, nil
}

func (s *TokenService) fillTokenUsers(tokens []domains.ApiToken) {
	userGuids := make([]string, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		guid := strings.TrimSpace(token.UserGuid)
		if guid == "" || seen[guid] {
			continue
		}
		seen[guid] = true
		userGuids = append(userGuids, guid)
	}
	if len(userGuids) == 0 {
		return
	}
	var users []commonDomains.SysUser
	if err := s.DB().Where("guid IN ?", userGuids).Find(&users).Error; err != nil {
		return
	}
	userMap := make(map[string]commonDomains.SysUser, len(users))
	for _, user := range users {
		userMap[user.Guid] = user
	}
	for i := range tokens {
		user, ok := userMap[tokens[i].UserGuid]
		if !ok {
			continue
		}
		tokens[i].Username = user.Username
		tokens[i].Email = user.Email
	}
}

func (s *TokenService) Validate(key string, clientIP string) (*domains.ApiToken, error) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "Bearer "))
	if key == "" {
		return nil, errors.New("token is required")
	}
	var token domains.ApiToken
	if err := s.DB().Where("key = ?", key).First(&token).Error; err != nil {
		return nil, err
	}
	if token.Status != constants.StatusEnabled {
		return nil, errors.New("token is disabled")
	}
	now := time.Now().Unix()
	if token.ExpiredTime > 0 && token.ExpiredTime < now {
		return nil, errors.New("token is expired")
	}
	if !token.UnlimitedBalance && effectiveTokenBalanceAmountMicros(&token) <= 0 {
		return nil, errors.New("token balance is exhausted")
	}
	if token.AllowIPs != "" && !containsString(splitCSV(token.AllowIPs), clientIP) {
		return nil, errors.New("client ip is not allowed")
	}
	token.AccessedTime = now
	_ = s.DB().Model(&domains.ApiToken{}).Where("id = ?", token.Id).Update("accessed_time", now).Error
	return &token, nil
}

func (s *TokenService) ResolveForBalance(key string, clientIP string) (*domains.ApiToken, error) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "Bearer "))
	if key == "" {
		return nil, errors.New("token is required")
	}
	var token domains.ApiToken
	if err := s.DB().Where("key = ?", key).First(&token).Error; err != nil {
		return nil, err
	}
	if token.Status != constants.StatusEnabled {
		return nil, errors.New("token is disabled")
	}
	now := time.Now().Unix()
	if token.ExpiredTime > 0 && token.ExpiredTime < now {
		return nil, errors.New("token is expired")
	}
	if token.AllowIPs != "" && !containsString(splitCSV(token.AllowIPs), clientIP) {
		return nil, errors.New("client ip is not allowed")
	}
	token.AccessedTime = now
	_ = s.DB().Model(&domains.ApiToken{}).Where("id = ?", token.Id).Update("accessed_time", now).Error
	return &token, nil
}

func (s *TokenService) CheckModel(token *domains.ApiToken, modelName string) error {
	if token == nil {
		return errors.New("token is required")
	}
	if err := ModelServiceApp.ModelAllowedForGroup(modelName, token.Group); err != nil {
		return err
	}
	if !token.ModelLimitsEnabled {
		return nil
	}
	if containsString(splitCSV(token.ModelLimits), modelName) {
		return nil
	}
	return errors.New("model is not allowed by token")
}

func (s *TokenService) ConsumeAmount(tx *gorm.DB, id uint, amountMicros int64) error {
	if amountMicros <= 0 {
		return nil
	}
	var token domains.ApiToken
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&token, id).Error; err != nil {
		return err
	}
	balance := effectiveTokenBalanceAmountMicros(&token)
	used := effectiveTokenUsedAmountMicros(&token)
	updates := map[string]any{
		"used_amount_micros": used + amountMicros,
	}
	if !token.UnlimitedBalance {
		if balance < amountMicros {
			return errors.New("token balance is exhausted")
		}
		updates["balance_amount_micros"] = balance - amountMicros
	}
	return tx.Model(&domains.ApiToken{}).Where("id = ?", id).Updates(updates).Error
}

func (s *TokenService) RefundAmount(tx *gorm.DB, id uint, amountMicros int64) error {
	if amountMicros <= 0 {
		return nil
	}
	var token domains.ApiToken
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&token, id).Error; err != nil {
		return err
	}
	balance := effectiveTokenBalanceAmountMicros(&token)
	updates := map[string]any{
		"used_amount_micros": gorm.Expr("CASE WHEN used_amount_micros >= ? THEN used_amount_micros - ? ELSE 0 END", amountMicros, amountMicros),
	}
	if !token.UnlimitedBalance {
		updates["balance_amount_micros"] = balance + amountMicros
	}
	return tx.Model(&domains.ApiToken{}).Where("id = ?", id).Updates(updates).Error
}

func (s *TokenService) AddAmount(tx *gorm.DB, id uint, userGuid string, amountMicros int64) error {
	if amountMicros <= 0 {
		return errors.New("amount must be greater than zero")
	}
	var token domains.ApiToken
	db := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if err := db.First(&token).Error; err != nil {
		return err
	}
	return tx.Model(&domains.ApiToken{}).Where("id = ?", token.Id).
		Updates(map[string]any{
			"balance_amount_micros": effectiveTokenBalanceAmountMicros(&token) + amountMicros,
		}).Error
}

func (s *TokenService) Usage(userGuid string) ([]TokenUsage, error) {
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
		db := s.DB().Model(&domains.UsageLog{}).
			Select("token_guid, COUNT(*) AS total_requests, COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END),0) AS success_requests, COALESCE(SUM(quota),0) AS quota, COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens").
			Where("token_guid IN ?", tokenGuids)
		db = applyUserUsageLogSourceFilter(db)
		if userGuid != "" {
			db = db.Where("user_guid = ?", userGuid)
		}
		if err := db.Group("token_guid").Scan(&rows).Error; err != nil {
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
		usage.UnlimitedBalance = token.UnlimitedBalance
		usage.BalanceAmountMicros = effectiveTokenBalanceAmountMicros(&token)
		usage.UsedAmountMicros = effectiveTokenUsedAmountMicros(&token)
		usage.AccessedTime = token.AccessedTime
		out = append(out, usage)
	}
	return out, nil
}

func (s *TokenService) Mask(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return key[:2] + "****"
	}
	return key[:6] + "********" + key[len(key)-4:]
}

func normalizeTokenAmounts(token *domains.ApiToken) {
	if token == nil {
		return
	}
	token.BalanceAmountMicros = nonNegativeInt64(token.BalanceAmountMicros)
	token.UsedAmountMicros = nonNegativeInt64(token.UsedAmountMicros)
}

func effectiveTokenBalanceAmountMicros(token *domains.ApiToken) int64 {
	if token == nil {
		return 0
	}
	return nonNegativeInt64(token.BalanceAmountMicros)
}

func effectiveTokenUsedAmountMicros(token *domains.ApiToken) int64 {
	if token == nil {
		return 0
	}
	return nonNegativeInt64(token.UsedAmountMicros)
}
