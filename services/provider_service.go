package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type ProviderService struct {
	commonServices.CrudService[domains.VendorMeta]
}

var ProviderServiceApp = new(ProviderService)

const (
	balanceTemplateGeneric = "generic"
	balanceTemplateNewAPI  = "newapi"
	balanceTemplateCustom  = "custom"

	balanceAuthProviderBearer = "provider_bearer"
)

type ProviderRecord struct {
	domains.VendorMeta
	HasKey           bool `json:"hasKey"`
	HasProxyPassword bool `json:"hasProxyPassword"`
}

type ProviderListQuery struct {
	vos.PageQuery
	Type      string `form:"type" json:"type"`
	Status    string `form:"status" json:"status"`
	KeyStatus string `form:"keyStatus" json:"keyStatus"`
}

type providerAffinityEntry struct {
	ProviderGuid string
	ExpiresAt    time.Time
}

type ProviderTestResult struct {
	OK           bool     `json:"ok"`
	ResponseTime int64    `json:"responseTime"`
	StatusCode   int      `json:"statusCode,omitempty"`
	Message      string   `json:"message,omitempty"`
	TargetURL    string   `json:"targetUrl,omitempty"`
	Models       []string `json:"models,omitempty"`
}

var providerKeyRotation = struct {
	sync.Mutex
	next map[string]int
}{next: map[string]int{}}

var providerAffinity = struct {
	sync.Mutex
	entries map[string]providerAffinityEntry
}{entries: map[string]providerAffinityEntry{}}

func (s *ProviderService) WithDB(db *gorm.DB) *ProviderService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func ProviderRecordFromDomain(provider domains.VendorMeta) ProviderRecord {
	hasProxyPassword := strings.TrimSpace(provider.ProxyPassword) != ""
	provider.ProxyPassword = ""
	return ProviderRecord{
		VendorMeta:       provider,
		HasKey:           strings.TrimSpace(provider.Key) != "",
		HasProxyPassword: hasProxyPassword,
	}
}

func (s *ProviderService) List(query ProviderListQuery) (vos.PageResult, error) {
	query.Normalize()
	var providers []domains.VendorMeta
	var total int64
	db := s.DB()
	if db == nil {
		return vos.PageResult{}, errors.New("database is not initialized")
	}
	db = db.Model(&domains.VendorMeta{})
	if query.Q != "" {
		db = db.Where("vendor_name LIKE ? OR display_name LIKE ? OR type LIKE ? OR base_url LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	providerType := strings.TrimSpace(query.Type)
	if providerType != "" {
		db = db.Where("type = ?", providerType)
	}
	switch strings.TrimSpace(query.Status) {
	case "enabled":
		db = db.Where("enabled = ?", true)
	case "disabled":
		db = db.Where("enabled = ?", false)
	}
	switch strings.TrimSpace(query.KeyStatus) {
	case "set":
		db = db.Where("TRIM(COALESCE(`key`, '')) <> ''")
	case "missing":
		db = db.Where("TRIM(COALESCE(`key`, '')) = ''")
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("sort desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&providers).Error; err != nil {
		return vos.PageResult{}, err
	}
	records := make([]ProviderRecord, 0, len(providers))
	for _, provider := range providers {
		records = append(records, ProviderRecordFromDomain(provider))
	}
	return vos.PageResult{List: records, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *ProviderService) GetByID(id uint) (*domains.VendorMeta, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	provider, err := s.GetById(id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("provider not found")
	}
	return provider, nil
}

func (s *ProviderService) GetByGUID(guid string) (*domains.VendorMeta, error) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil, errors.New("guid is required")
	}
	provider, err := s.GetByGuid(guid)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("provider not found")
	}
	return provider, nil
}

// Save normalizes provider defaults and validates JSON override fields before
// storing the upstream configuration used by relay routing.
func (s *ProviderService) Save(provider *domains.VendorMeta) error {
	requestedEnabled := provider.Enabled
	provider.VendorName = strings.TrimSpace(provider.VendorName)
	provider.DisplayName = strings.TrimSpace(provider.DisplayName)
	provider.Type = strings.TrimSpace(provider.Type)
	provider.LogoURL = strings.TrimSpace(provider.LogoURL)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.Models = strings.Join(splitCSV(provider.Models), ",")
	provider.ModelOverride = strings.TrimSpace(provider.ModelOverride)
	provider.QuotaModelWhitelist = strings.Join(splitCSV(provider.QuotaModelWhitelist), ",")
	provider.ModelMapping = strings.TrimSpace(provider.ModelMapping)
	provider.HeaderOverride = strings.TrimSpace(provider.HeaderOverride)
	provider.ParamOverride = strings.TrimSpace(provider.ParamOverride)
	normalizeProviderProxyConfig(provider)
	normalizeProviderBalanceConfig(provider)
	provider.Website = strings.TrimSpace(provider.Website)
	provider.Remark = strings.TrimSpace(provider.Remark)
	provider.Key = strings.TrimSpace(provider.Key)
	if strings.TrimSpace(provider.VendorName) == "" {
		return errors.New("provider name is required")
	}
	if strings.TrimSpace(provider.Type) == "" {
		provider.Type = constants.ProviderTypeOpenAI
	}
	if provider.DisplayName == "" {
		provider.DisplayName = provider.VendorName
	}
	if err := validateOptionalJSONObject(provider.ModelMapping, "modelMapping"); err != nil {
		return err
	}
	if err := validateOptionalJSONObject(provider.HeaderOverride, "headerOverride"); err != nil {
		return err
	}
	if err := validateOptionalJSONObject(provider.ParamOverride, "paramOverride"); err != nil {
		return err
	}
	if err := validateProviderProxyConfig(provider); err != nil {
		return err
	}
	var existing *domains.VendorMeta
	var err error
	if provider.Guid != "" {
		existing, err = s.GetByGUID(provider.Guid)
	} else if provider.Id != 0 {
		existing, err = s.GetByID(provider.Id)
	}
	if err != nil {
		return err
	}
	if existing == nil {
		if err := createWithCrud(&s.CrudService, provider); err != nil {
			return err
		}
		return s.setEnabled(provider, requestedEnabled)
	}
	provider.Guid = existing.Guid
	provider.CreateTime = existing.CreateTime
	provider.Creater = existing.Creater
	provider.Updater = existing.Updater
	if provider.Key == "" {
		provider.Key = existing.Key
	}
	if provider.ProxyPassword == "" {
		provider.ProxyPassword = existing.ProxyPassword
	}
	provider.Id = existing.Id
	provider.UpdateTime = time.Now().UnixMilli()
	if err := s.DB().Save(provider).Error; err != nil {
		return err
	}
	if err := reloadByGuidWithCrud(&s.CrudService, provider); err != nil {
		return err
	}
	return s.setEnabled(provider, requestedEnabled)
}

func (s *ProviderService) setEnabled(provider *domains.VendorMeta, enabled bool) error {
	db := s.DB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	if err := db.Model(&domains.VendorMeta{}).Where("guid = ?", provider.Guid).Update("enabled", enabled).Error; err != nil {
		return err
	}
	provider.Enabled = enabled
	return nil
}

func (s *ProviderService) Delete(guid string) error {
	provider, err := s.GetByGUID(guid)
	if err != nil {
		return err
	}
	return s.DeleteByGuid(provider.Guid)
}

func (s *ProviderService) GetKey(guid string) (string, error) {
	provider, err := s.GetByGUID(guid)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(provider.Key) == "" {
		return "", errors.New("provider key is empty")
	}
	return provider.Key, nil
}

func (s *ProviderService) SetKey(guid string, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("provider key is required")
	}
	provider, err := s.GetByGUID(guid)
	if err != nil {
		return err
	}
	return s.Update(*provider, "key", key)
}

func normalizeProviderBalanceConfig(provider *domains.VendorMeta) {
	provider.BalanceTemplate = normalizeBalanceTemplate(provider.BalanceTemplate)
	provider.BalanceBaseURL = strings.TrimSpace(provider.BalanceBaseURL)
	provider.BalanceAccessToken = strings.TrimSpace(provider.BalanceAccessToken)
	provider.BalanceUserID = strings.TrimSpace(provider.BalanceUserID)
	provider.BalanceCustomPath = strings.TrimSpace(provider.BalanceCustomPath)
	if provider.BalanceCustomPath == "" || provider.BalanceCustomPath == "/v1/usage" {
		provider.BalanceCustomPath = defaultBalancePath(provider.BalanceTemplate)
	}
	provider.BalanceAuthType = defaultString(provider.BalanceAuthType, balanceAuthProviderBearer)
	provider.BalanceRemainingPath = defaultString(provider.BalanceRemainingPath, defaultBalanceRemainingPath(provider.BalanceTemplate))
	if provider.BalanceMultiplier <= 0 {
		provider.BalanceMultiplier = defaultBalanceMultiplier(provider.BalanceTemplate)
	}
	provider.BalanceUnit = defaultString(provider.BalanceUnit, defaultBalanceUnit(provider.BalanceTemplate))
	provider.BalanceTotalPath = strings.TrimSpace(provider.BalanceTotalPath)
	provider.BalanceUsedPath = strings.TrimSpace(provider.BalanceUsedPath)
	provider.BalancePlanPath = strings.TrimSpace(provider.BalancePlanPath)
	provider.BalanceValidPath = strings.TrimSpace(provider.BalanceValidPath)
	provider.BalanceErrorPath = strings.TrimSpace(provider.BalanceErrorPath)
}

func (s *ProviderService) ListEnabledModels() ([]string, error) {
	providers, err := s.enabledProviders("")
	if err != nil {
		return nil, err
	}
	modelSet := map[string]struct{}{}
	for _, provider := range providers {
		for _, model := range splitCSV(provider.Models) {
			modelSet[model] = struct{}{}
		}
	}
	out := make([]string, 0, len(modelSet))
	for model := range modelSet {
		out = append(out, model)
	}
	sort.Strings(out)
	return out, nil
}

func (s *ProviderService) FindCandidatesForModelAndType(modelName, group string, providerType string) ([]domains.VendorMeta, error) {
	_ = group
	providers, err := s.enabledProviders(providerType)
	if err != nil {
		return nil, err
	}
	candidates := make([]domains.VendorMeta, 0, len(providers))
	for _, provider := range providers {
		if len(splitCSV(provider.Models)) > 0 && !containsString(splitCSV(provider.Models), modelName) {
			continue
		}
		candidates = append(candidates, provider)
	}
	if len(candidates) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return candidates, nil
}

func (s *ProviderService) ApplyAffinity(tokenGuid string, modelName string, candidates []domains.VendorMeta) []domains.VendorMeta {
	ttl := OptionServiceApp.Int64("relay.provider_affinity_seconds", 0)
	if ttl <= 0 || tokenGuid == "" || modelName == "" || len(candidates) <= 1 {
		return candidates
	}
	key := tokenGuid + ":" + modelName
	providerAffinity.Lock()
	entry, ok := providerAffinity.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		delete(providerAffinity.entries, key)
		providerAffinity.Unlock()
		return candidates
	}
	providerAffinity.Unlock()
	for i, candidate := range candidates {
		if candidate.Guid != entry.ProviderGuid {
			continue
		}
		out := make([]domains.VendorMeta, 0, len(candidates))
		out = append(out, candidate)
		out = append(out, candidates[:i]...)
		out = append(out, candidates[i+1:]...)
		return out
	}
	return candidates
}

func (s *ProviderService) RememberAffinity(tokenGuid string, modelName string, providerGuid string) {
	ttl := OptionServiceApp.Int64("relay.provider_affinity_seconds", 0)
	if ttl <= 0 || tokenGuid == "" || modelName == "" || providerGuid == "" {
		return
	}
	key := tokenGuid + ":" + modelName
	providerAffinity.Lock()
	providerAffinity.entries[key] = providerAffinityEntry{
		ProviderGuid: providerGuid,
		ExpiresAt:    time.Now().Add(time.Duration(ttl) * time.Second),
	}
	providerAffinity.Unlock()
}

func (s *ProviderService) MapModel(provider *domains.VendorMeta, modelName string) string {
	if provider != nil {
		if override := strings.TrimSpace(provider.ModelOverride); override != "" {
			return override
		}
	}
	if provider == nil || strings.TrimSpace(provider.ModelMapping) == "" || strings.TrimSpace(modelName) == "" {
		return modelName
	}
	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(provider.ModelMapping), &mapping); err != nil {
		return modelName
	}
	if mapped := strings.TrimSpace(mapping[modelName]); mapped != "" {
		return mapped
	}
	return modelName
}

func (s *ProviderService) AutoDisable(guid string, reason string) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil
	}
	if len(reason) > 255 {
		reason = reason[:255]
	}
	return s.DB().Model(&domains.VendorMeta{}).Where("guid = ?", guid).
		Updates(map[string]any{"enabled": false, "remark": reason}).Error
}

func (s *ProviderService) Test(guid string) (*ProviderTestResult, error) {
	provider, err := s.GetByGUID(guid)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	models, err := s.fetchModels(provider)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	return &ProviderTestResult{OK: true, ResponseTime: responseTime, Models: models}, nil
}

func (s *ProviderService) TestConnection(provider *domains.VendorMeta) (*ProviderTestResult, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	provider.Type = strings.TrimSpace(provider.Type)
	if provider.Type == "" {
		provider.Type = constants.ProviderTypeOpenAI
	}
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	if provider.BaseURL == "" {
		return nil, errors.New("base url is required")
	}
	if strings.TrimSpace(provider.Guid) != "" {
		if existing, err := s.GetByGUID(provider.Guid); err == nil && existing != nil {
			if strings.TrimSpace(provider.Key) == "" {
				provider.Key = existing.Key
			}
			if strings.TrimSpace(provider.ProxyPassword) == "" {
				provider.ProxyPassword = existing.ProxyPassword
			}
		}
	}
	normalizeProviderProxyConfig(provider)
	if err := validateProviderProxyConfig(provider); err != nil {
		return nil, err
	}
	targetURL, err := normalizeProviderTestURL(provider.BaseURL)
	if err != nil {
		return nil, err
	}
	client, err := providerHTTPClient(provider, 8*time.Second)
	if err != nil {
		return nil, err
	}
	result := doProviderProbe(client, http.MethodHead, targetURL)
	if result.StatusCode == http.StatusMethodNotAllowed {
		result = doProviderProbe(client, http.MethodGet, targetURL)
	}
	return result, nil
}

func (s *ProviderService) FetchModels(guid string, update bool) ([]string, error) {
	provider, err := s.GetByGUID(guid)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchModels(provider)
	if err != nil {
		return nil, err
	}
	models = uniqueSorted(models)
	if update {
		if err := s.DB().Model(&domains.VendorMeta{}).Where("guid = ?", provider.Guid).
			Update("models", strings.Join(models, ",")).Error; err != nil {
			return nil, err
		}
	}
	return models, nil
}

func normalizeProviderTestURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("base url is required")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("base url is invalid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", errors.New("base url only supports http and https")
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func doProviderProbe(client *http.Client, method string, targetURL string) *ProviderTestResult {
	start := time.Now()
	req, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return &ProviderTestResult{OK: false, Message: err.Error(), TargetURL: targetURL}
	}
	req.Header.Set("User-Agent", "NavAPI Gateway Probe")
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	resp, err := client.Do(req)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		return &ProviderTestResult{
			OK:           false,
			ResponseTime: responseTime,
			Message:      err.Error(),
			TargetURL:    targetURL,
		}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	ok := resp.StatusCode < http.StatusInternalServerError
	message := "connected"
	if !ok {
		message = fmt.Sprintf("upstream returned %d", resp.StatusCode)
	}
	return &ProviderTestResult{
		OK:           ok,
		ResponseTime: responseTime,
		StatusCode:   resp.StatusCode,
		Message:      message,
		TargetURL:    targetURL,
	}
}

func (s *ProviderService) NextKey(provider *domains.VendorMeta) string {
	if provider == nil {
		return ""
	}
	keys := splitCSV(provider.Key)
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 {
		return keys[0]
	}
	rotationKey := provider.Guid
	if rotationKey == "" {
		rotationKey = provider.VendorName
	}
	providerKeyRotation.Lock()
	defer providerKeyRotation.Unlock()
	idx := providerKeyRotation.next[rotationKey] % len(keys)
	providerKeyRotation.next[rotationKey] = idx + 1
	return keys[idx]
}

func (s *ProviderService) enabledProviders(providerType string) ([]domains.VendorMeta, error) {
	var providers []domains.VendorMeta
	db := s.DB().Where("enabled = ? AND TRIM(COALESCE(`key`, '')) <> ''", true)
	if strings.TrimSpace(providerType) != "" {
		db = db.Where("type = ?", strings.TrimSpace(providerType))
	}
	err := db.Order("sort desc, id desc").Find(&providers).Error
	return providers, err
}

func (s *ProviderService) fetchModels(provider *domains.VendorMeta) ([]string, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	path := "/v1/models"
	switch provider.Type {
	case constants.ProviderTypeAnthropic:
		path = "/v1/models"
	case constants.ProviderTypeGemini:
		path = "/v1beta/models"
	}
	targetURL := strings.TrimRight(provider.BaseURL, "/")
	if targetURL == "" {
		targetURL = defaultBaseURL(provider.Type)
	}
	targetURL += path
	if provider.Type == constants.ProviderTypeGemini {
		targetURL = attachGeminiKey(targetURL, provider.Key, "")
	}
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	setupAuthHeaders(req.Header, provider)
	client, err := providerHTTPClient(provider, 30*time.Second)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if len(body) > 500 {
			body = body[:500]
		}
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}
	return parseModelIDs(provider.Type, body), nil
}

func normalizeBalanceTemplate(template string) string {
	switch strings.ToLower(strings.TrimSpace(template)) {
	case balanceTemplateNewAPI:
		return balanceTemplateNewAPI
	case balanceTemplateOfficial:
		return balanceTemplateOfficial
	case balanceTemplateCustom:
		return balanceTemplateGeneric
	default:
		return balanceTemplateGeneric
	}
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
