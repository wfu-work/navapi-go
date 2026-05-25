package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
)

type ChannelService struct{}

var ChannelServiceApp = ChannelService{}

var channelKeyRotation = struct {
	sync.Mutex
	next map[string]int
}{next: map[string]int{}}

type channelAffinityEntry struct {
	ChannelID uint
	ExpiresAt time.Time
}

var channelAffinity = struct {
	sync.Mutex
	entries map[string]channelAffinityEntry
}{entries: map[string]channelAffinityEntry{}}

type ChannelTestResult struct {
	OK           bool     `json:"ok"`
	ResponseTime int64    `json:"responseTime"`
	Models       []string `json:"models,omitempty"`
}

func (s ChannelService) Create(channel *domains.Channel) error {
	channel.Group = normalizeGroup(channel.Group)
	if channel.Type == "" {
		channel.Type = constants.ChannelTypeOpenAI
	}
	if channel.Weight <= 0 {
		channel.Weight = 1
	}
	return global.NAV_DB.Create(channel).Error
}

func (s ChannelService) Update(channel *domains.Channel) error {
	return global.NAV_DB.Save(channel).Error
}

func (s ChannelService) Delete(id uint) error {
	return global.NAV_DB.Delete(&domains.Channel{}, id).Error
}

func (s ChannelService) GetByID(id uint) (*domains.Channel, error) {
	var channel domains.Channel
	if err := global.NAV_DB.First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (s ChannelService) List() ([]domains.Channel, error) {
	var channels []domains.Channel
	err := global.NAV_DB.Order("priority desc, id desc").Find(&channels).Error
	return channels, err
}

func (s ChannelService) ListEnabledModels() ([]string, error) {
	channels, err := s.List()
	if err != nil {
		return nil, err
	}
	modelSet := map[string]struct{}{}
	for _, channel := range channels {
		if channel.Status != constants.StatusEnabled {
			continue
		}
		for _, model := range splitCSV(channel.Models) {
			modelSet[model] = struct{}{}
		}
	}
	out := make([]string, 0, len(modelSet))
	for model := range modelSet {
		out = append(out, model)
	}
	return out, nil
}

func (s ChannelService) FindForModel(modelName, group string) (*domains.Channel, error) {
	return s.FindForModelAndType(modelName, group, "")
}

func (s ChannelService) FindForModelAndType(modelName, group string, channelType string) (*domains.Channel, error) {
	candidates, err := s.FindCandidatesForModelAndType(modelName, group, channelType)
	if err != nil {
		return nil, err
	}
	return &candidates[0], nil
}

func (s ChannelService) FindCandidatesForModelAndType(modelName, group string, channelType string) ([]domains.Channel, error) {
	group = normalizeGroup(group)
	var channels []domains.Channel
	db := global.NAV_DB.Where("status = ? AND (group_name = ? OR group_name = ?)", constants.StatusEnabled, group, "default")
	if channelType != "" {
		db = db.Where("type = ?", channelType)
	}
	err := db.
		Order("priority desc, id desc").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	candidates := make([]domains.Channel, 0, len(channels))
	for _, channel := range channels {
		if len(splitCSV(channel.Models)) > 0 && !containsString(splitCSV(channel.Models), modelName) {
			continue
		}
		candidates = append(candidates, channel)
	}
	if len(candidates) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return weightedChannelOrder(candidates), nil
}

func (s ChannelService) ApplyAffinity(tokenGuid string, modelName string, candidates []domains.Channel) []domains.Channel {
	ttl := OptionServiceApp.Int64("relay.channel_affinity_seconds", 0)
	if ttl <= 0 || tokenGuid == "" || modelName == "" || len(candidates) <= 1 {
		return candidates
	}
	key := tokenGuid + ":" + modelName
	channelAffinity.Lock()
	entry, ok := channelAffinity.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		delete(channelAffinity.entries, key)
		channelAffinity.Unlock()
		return candidates
	}
	channelAffinity.Unlock()
	for i, candidate := range candidates {
		if candidate.Id != entry.ChannelID {
			continue
		}
		out := make([]domains.Channel, 0, len(candidates))
		out = append(out, candidate)
		out = append(out, candidates[:i]...)
		out = append(out, candidates[i+1:]...)
		return out
	}
	return candidates
}

func (s ChannelService) RememberAffinity(tokenGuid string, modelName string, channelID uint) {
	ttl := OptionServiceApp.Int64("relay.channel_affinity_seconds", 0)
	if ttl <= 0 || tokenGuid == "" || modelName == "" || channelID == 0 {
		return
	}
	key := tokenGuid + ":" + modelName
	channelAffinity.Lock()
	channelAffinity.entries[key] = channelAffinityEntry{
		ChannelID: channelID,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}
	channelAffinity.Unlock()
}

func weightedChannelOrder(candidates []domains.Channel) []domains.Channel {
	ordered := make([]domains.Channel, 0, len(candidates))
	pool := append([]domains.Channel(nil), candidates...)
	rand.Seed(time.Now().UnixNano())
	for len(pool) > 0 {
		idx := pickWeightedChannel(pool)
		ordered = append(ordered, pool[idx])
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	return ordered
}

func pickWeightedChannel(candidates []domains.Channel) int {
	weightSum := 0
	for _, channel := range candidates {
		weightSum += intMax(1, channel.Weight)
	}
	if weightSum <= 0 {
		return 0
	}
	pick := rand.Intn(weightSum)
	for i := range candidates {
		pick -= intMax(1, candidates[i].Weight)
		if pick < 0 {
			return i
		}
	}
	return 0
}

func (s ChannelService) MapModel(channel *domains.Channel, modelName string) string {
	if channel == nil || strings.TrimSpace(channel.ModelMapping) == "" || strings.TrimSpace(modelName) == "" {
		return modelName
	}
	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(channel.ModelMapping), &mapping); err != nil {
		return modelName
	}
	if mapped := strings.TrimSpace(mapping[modelName]); mapped != "" {
		return mapped
	}
	return modelName
}

func (s ChannelService) MatchModel(channel *domains.Channel, modelName string) bool {
	if channel == nil {
		return false
	}
	models := splitCSV(channel.Models)
	if len(models) == 0 {
		return true
	}
	return containsString(models, modelName)
}

func (s ChannelService) IncreaseUsage(id uint, quota int64) error {
	return global.NAV_DB.Model(&domains.Channel{}).Where("id = ?", id).
		UpdateColumn("used_quota", gorm.Expr("used_quota + ?", quota)).Error
}

func (s ChannelService) SetTestResult(id uint, responseTime int64) error {
	return global.NAV_DB.Model(&domains.Channel{}).Where("id = ?", id).
		Updates(map[string]any{
			"test_time":     time.Now().Unix(),
			"response_time": responseTime,
		}).Error
}

func (s ChannelService) SetStatus(id uint, status int) error {
	return global.NAV_DB.Model(&domains.Channel{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "disabled_reason": ""}).Error
}

func (s ChannelService) AutoDisable(id uint, reason string) error {
	if len(reason) > 255 {
		reason = reason[:255]
	}
	return global.NAV_DB.Model(&domains.Channel{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":          constants.StatusDisabled,
			"disabled_reason": reason,
		}).Error
}

func (s ChannelService) BatchStatus(ids []uint, status int) error {
	if len(ids) == 0 {
		return errors.New("ids is required")
	}
	if status != constants.StatusEnabled && status != constants.StatusDisabled {
		return errors.New("invalid status")
	}
	return global.NAV_DB.Model(&domains.Channel{}).Where("id IN ?", ids).
		Update("status", status).Error
}

func (s ChannelService) SetStatusByTag(tag string, status int) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return errors.New("tag is required")
	}
	if status != constants.StatusEnabled && status != constants.StatusDisabled {
		return errors.New("invalid status")
	}
	var channels []domains.Channel
	if err := global.NAV_DB.Select("id", "tags").Find(&channels).Error; err != nil {
		return err
	}
	ids := make([]uint, 0)
	for _, channel := range channels {
		if containsString(splitCSV(channel.Tags), tag) {
			ids = append(ids, channel.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return global.NAV_DB.Model(&domains.Channel{}).Where("id IN ?", ids).
		Update("status", status).Error
}

func (s ChannelService) Test(id uint) (*ChannelTestResult, error) {
	start := time.Now()
	models, err := s.FetchModels(id, false)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		_ = s.SetTestResult(id, responseTime)
		return nil, err
	}
	if err := s.SetTestResult(id, responseTime); err != nil {
		return nil, err
	}
	return &ChannelTestResult{OK: true, ResponseTime: responseTime, Models: models}, nil
}

func (s ChannelService) FetchModels(id uint, update bool) ([]string, error) {
	channel, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchModels(channel)
	if err != nil {
		return nil, err
	}
	models = uniqueSorted(models)
	if update {
		if err := global.NAV_DB.Model(&domains.Channel{}).Where("id = ?", id).
			Update("models", strings.Join(models, ",")).Error; err != nil {
			return nil, err
		}
	}
	return models, nil
}

func (s ChannelService) fetchModels(channel *domains.Channel) ([]string, error) {
	if channel == nil {
		return nil, errors.New("channel is required")
	}
	path := "/v1/models"
	switch channel.Type {
	case constants.ChannelTypeAnthropic:
		path = "/v1/models"
	case constants.ChannelTypeGemini:
		path = "/v1beta/models"
	}
	targetURL := strings.TrimRight(channel.BaseURL, "/")
	if targetURL == "" {
		targetURL = defaultBaseURL(channel.Type)
	}
	targetURL += path
	if channel.Type == constants.ChannelTypeGemini {
		targetURL = attachGeminiKeyForChannel(targetURL, channel.Key)
	}
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	setupAuthHeaders(req.Header, channel)
	client := &http.Client{Timeout: 30 * time.Second}
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
	return parseModelIDs(channel.Type, body), nil
}

func (s ChannelService) GetChannelKey(id uint) (string, error) {
	channel, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(channel.Key) == "" {
		return "", errors.New("channel key is empty")
	}
	return channel.Key, nil
}

func (s ChannelService) NextKey(channel *domains.Channel) string {
	if channel == nil {
		return ""
	}
	keys := splitCSV(channel.Key)
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 {
		return keys[0]
	}
	rotationKey := channel.Guid
	if rotationKey == "" {
		rotationKey = fmt.Sprint(channel.Id)
	}
	channelKeyRotation.Lock()
	defer channelKeyRotation.Unlock()
	idx := channelKeyRotation.next[rotationKey] % len(keys)
	channelKeyRotation.next[rotationKey] = idx + 1
	return keys[idx]
}

func parseModelIDs(channelType string, body []byte) []string {
	switch channelType {
	case constants.ChannelTypeGemini:
		return parseGeminiModelIDs(body)
	default:
		return parseDataModelIDs(body)
	}
}

func parseDataModelIDs(body []byte) []string {
	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, item.ID)
			continue
		}
		if item.Name != "" {
			models = append(models, item.Name)
			continue
		}
		if item.DisplayName != "" {
			models = append(models, item.DisplayName)
		}
	}
	return models
}

func parseGeminiModelIDs(body []byte) []string {
	var payload struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	models := make([]string, 0, len(payload.Models))
	for _, item := range payload.Models {
		name := strings.TrimPrefix(item.Name, "models/")
		if name != "" {
			models = append(models, name)
			continue
		}
		if item.DisplayName != "" {
			models = append(models, item.DisplayName)
		}
	}
	return models
}

func uniqueSorted(models []string) []string {
	set := map[string]struct{}{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			set[model] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for model := range set {
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}

func attachGeminiKeyForChannel(targetURL string, key string) string {
	u, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}
	query := u.Query()
	if query.Get("key") == "" {
		query.Set("key", strings.TrimSpace(key))
	}
	u.RawQuery = query.Encode()
	return u.String()
}
