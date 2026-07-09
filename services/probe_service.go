package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/robfig/cron/v3"
	commonScheduleds "github.com/wfu-work/nav-common-go-lib/scheduleds"
	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

const (
	settingProbeEnabled               = "probe.enabled"
	settingProbeIntervalSeconds       = "probe.interval_seconds"
	settingProbeRetentionDays         = "probe.retention_days"
	legacySettingProbeEnabled         = "service_probe.enabled"
	legacySettingProbeIntervalSeconds = "service_probe.interval_seconds"
	legacySettingProbeRetentionDays   = "service_probe.retention_days"
	probeCronName                     = "navapi"
	probeTaskName                     = "navapi_probe"
	defaultProbeIntervalSeconds       = int64(60)
	defaultProbeRetentionDays         = int64(30)
	minProbeIntervalSeconds           = int64(60)
	minProbeRetentionDays             = int64(1)
	maxProbeRetentionDays             = int64(3650)
	probeTimeout                      = 45 * time.Second
)

type ProbeSettings struct {
	Enabled         bool  `json:"enabled"`
	IntervalSeconds int64 `json:"intervalSeconds"`
	RetentionDays   int64 `json:"retentionDays"`
	LastProbedAt    int64 `json:"lastProbedAt"`
}

type ProbeService struct {
	commonServices.CrudService[domains.ProbeLog]
	runMu          sync.Mutex
	schedulerMu    sync.Mutex
	timers         commonScheduleds.Timer
	cronOptions    []cron.Option
	taskRegistered bool
	taskSpec       string
	cancelRun      context.CancelFunc
}

var ProbeServiceApp = new(ProbeService)

func (s *ProbeService) WithDB(db *gorm.DB) *ProbeService {
	cloned := &ProbeService{}
	cloned.CrudService = *s.CrudService.WithDB(db)
	return cloned
}

func (s *ProbeService) Settings() ProbeSettings {
	return ProbeSettings{
		Enabled:         s.Enabled(),
		IntervalSeconds: s.IntervalSeconds(),
		RetentionDays:   s.RetentionDays(),
		LastProbedAt:    s.LastProbedAt(),
	}
}

func (s *ProbeService) Enabled() bool {
	return probeOptionInt64(settingProbeEnabled, legacySettingProbeEnabled, 1) > 0
}

func (s *ProbeService) IntervalSeconds() int64 {
	return normalizeProbeIntervalSeconds(probeOptionInt64(settingProbeIntervalSeconds, legacySettingProbeIntervalSeconds, defaultProbeIntervalSeconds))
}

func (s *ProbeService) RetentionDays() int64 {
	return normalizeProbeRetentionDays(probeOptionInt64(settingProbeRetentionDays, legacySettingProbeRetentionDays, defaultProbeRetentionDays))
}

func probeOptionInt64(key string, legacyKey string, fallback int64) int64 {
	if strings.TrimSpace(OptionServiceApp.Get(key, "")) != "" {
		return OptionServiceApp.Int64(key, fallback)
	}
	return OptionServiceApp.Int64(legacyKey, fallback)
}

func normalizeProbeIntervalSeconds(value int64) int64 {
	if value <= 0 {
		return defaultProbeIntervalSeconds
	}
	if value < minProbeIntervalSeconds {
		return minProbeIntervalSeconds
	}
	return value
}

func normalizeProbeRetentionDays(value int64) int64 {
	if value <= 0 {
		return defaultProbeRetentionDays
	}
	if value < minProbeRetentionDays {
		return minProbeRetentionDays
	}
	if value > maxProbeRetentionDays {
		return maxProbeRetentionDays
	}
	return value
}

// LastProbedAt returns the newest probe log time. The scheduler uses it to
// avoid duplicate probes when an immediate trigger and cron tick are close.
func (s *ProbeService) LastProbedAt() int64 {
	db := s.DB()
	if db == nil {
		return 0
	}
	var last int64
	_ = db.Model(&domains.ProbeLog{}).Select("COALESCE(MAX(create_time), 0)").Scan(&last).Error
	return last
}

func (s *ProbeService) EnsureIndexes() error {
	db := s.DB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	indexes := []struct {
		name string
		sql  string
	}{
		{name: "idx_nav_api_probe_model_time", sql: "CREATE INDEX idx_nav_api_probe_model_time ON nav_api_probe_logs(model_name, create_time)"},
		{name: "idx_nav_api_probe_status_time", sql: "CREATE INDEX idx_nav_api_probe_status_time ON nav_api_probe_logs(status, create_time)"},
	}
	for _, index := range indexes {
		if db.Migrator().HasIndex(&domains.ProbeLog{}, index.name) {
			continue
		}
		if err := db.Exec(index.sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// ConfigureScheduler is called once during application startup. It stores the
// common scheduler instance, then reconciles the task with current settings.
func (s *ProbeService) ConfigureScheduler(timers commonScheduleds.Timer, options []cron.Option) error {
	s.schedulerMu.Lock()
	s.timers = timers
	s.cronOptions = append([]cron.Option(nil), options...)
	s.schedulerMu.Unlock()
	return s.RefreshSchedule()
}

// RefreshSchedule keeps the probe task aligned with the latest switch and
// interval settings. It is safe to call after settings are saved or reloaded.
func (s *ProbeService) RefreshSchedule() error {
	enabled := s.Enabled()
	spec := fmt.Sprintf("@every %ds", s.IntervalSeconds())

	s.schedulerMu.Lock()
	defer s.schedulerMu.Unlock()
	if s.timers == nil {
		return nil
	}
	if !enabled {
		if s.taskRegistered {
			s.timers.RemoveTaskByName(probeCronName, probeTaskName)
			s.taskRegistered = false
			s.taskSpec = ""
		}
		s.cancelRunning()
		resetGatewayStatusCache()
		return nil
	}
	if s.taskRegistered && s.taskSpec == spec {
		return nil
	}
	if s.taskRegistered {
		s.timers.RemoveTaskByName(probeCronName, probeTaskName)
	}
	_, err := s.timers.AddTaskByFunc(probeCronName, spec, func() {
		_ = s.RunDue(context.Background())
	}, probeTaskName, s.cronOptions...)
	if err != nil {
		s.taskRegistered = false
		s.taskSpec = ""
		return err
	}
	s.taskRegistered = true
	s.taskSpec = spec
	s.triggerNow()
	return nil
}

// RunDue executes only when enough time has passed since the latest probe log.
func (s *ProbeService) RunDue(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	if !s.due(time.Now()) {
		return nil
	}
	return s.run(ctx)
}

// RunNow is used after enabling or changing the task so service status is
// refreshed without waiting for the next cron tick.
func (s *ProbeService) RunNow(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	return s.run(ctx)
}

// run serializes probes and binds the current execution to a cancel function so
// disabling the switch can stop an in-flight probe quickly.
func (s *ProbeService) run(ctx context.Context) error {
	if !s.runMu.TryLock() {
		return nil
	}
	defer s.runMu.Unlock()
	if !s.Enabled() {
		return nil
	}
	runCtx, done := s.runContext(ctx)
	defer done()
	return s.RunAll(runCtx)
}

func (s *ProbeService) runContext(ctx context.Context) (context.Context, func()) {
	runCtx, cancel := context.WithCancel(ctx)
	s.schedulerMu.Lock()
	s.cancelRun = cancel
	s.schedulerMu.Unlock()
	return runCtx, func() {
		cancel()
		s.schedulerMu.Lock()
		s.cancelRun = nil
		s.schedulerMu.Unlock()
	}
}

func (s *ProbeService) cancelRunning() {
	if s.cancelRun != nil {
		s.cancelRun()
		s.cancelRun = nil
	}
}

func (s *ProbeService) triggerNow() {
	go func() {
		_ = s.RunNow(context.Background())
	}()
}

func (s *ProbeService) due(now time.Time) bool {
	last := s.LastProbedAt()
	if last <= 0 {
		return true
	}
	return now.UnixMilli()-last >= s.IntervalSeconds()*1000
}

func (s *ProbeService) RunAll(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	models, err := ModelServiceApp.PublicListMeta()
	if err != nil {
		return err
	}
	for _, model := range models {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !s.Enabled() {
			return nil
		}
		if strings.TrimSpace(model.ModelName) == "" {
			continue
		}
		log := s.ProbeModel(ctx, model.ModelName)
		if ctx.Err() != nil || !s.Enabled() {
			return ctx.Err()
		}
		if err := s.DB().Create(&log).Error; err != nil {
			return err
		}
	}
	resetGatewayStatusCache()
	return nil
}

func (s *ProbeService) ProbeModel(ctx context.Context, modelName string) domains.ProbeLog {
	modelName = strings.TrimSpace(modelName)
	log := domains.ProbeLog{ModelName: modelName, Status: "error", Content: "no available provider"}
	if modelName == "" {
		log.Content = "model is required"
		return log
	}
	candidates, err := ProviderServiceApp.FindCandidatesForModelAndType(modelName, "", "")
	if err != nil || len(candidates) == 0 {
		if err != nil {
			log.Content = err.Error()
		}
		return log
	}
	var last domains.ProbeLog
	for index := range candidates {
		current := candidates[index]
		last = s.probeProvider(ctx, modelName, &current)
		if last.Status == "success" {
			return last
		}
	}
	return last
}

func (s *ProbeService) probeProvider(ctx context.Context, modelName string, provider *domains.VendorMeta) domains.ProbeLog {
	log := newProbeLog(modelName, provider)
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	method, upstreamPath, body, err := probeRequest(provider, modelName)
	if err != nil {
		log.Content = err.Error()
		return log
	}
	targetURL := probeURL(provider, upstreamPath)
	req, err := http.NewRequestWithContext(probeCtx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		log.Content = err.Error()
		return log
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NavAPI Probe")
	setupAuthHeaders(req.Header, provider)
	applyHeaderOverride(req.Header, provider.HeaderOverride)

	client, err := providerHTTPClient(provider, probeTimeout)
	if err != nil {
		log.Content = err.Error()
		return log
	}
	start := time.Now()
	resp, err := client.Do(req)
	log.UseTimeMs = time.Since(start).Milliseconds()
	if err != nil {
		log.Content = err.Error()
		return log
	}
	log.FirstResponseTimeMs = time.Since(start).Milliseconds()
	defer resp.Body.Close()
	log.StatusCode = resp.StatusCode
	bodyBytes, err := readLimitedUpstreamBody(resp)
	log.UseTimeMs = time.Since(start).Milliseconds()
	if err != nil {
		log.Content = err.Error()
		return log
	}
	usage := parseUsage(bodyBytes, resp.Header.Get("Content-Type"))
	log.PromptTokens = usage.PromptTokens
	log.CompletionTokens = usage.CompletionTokens
	log.UpstreamRequestID = extractUpstreamRequestID(&RelayResult{Header: resp.Header.Clone()})
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
		log.Status = "success"
		log.Content = "probe ok"
		return log
	}
	log.Content = probeBodySnippet(resp.StatusCode, bodyBytes)
	return log
}

func newProbeLog(modelName string, provider *domains.VendorMeta) domains.ProbeLog {
	log := domains.ProbeLog{ModelName: modelName, Status: "error"}
	if provider == nil {
		return log
	}
	log.ProviderGuid = provider.Guid
	log.ProviderType = strings.TrimSpace(provider.Type)
	log.ProviderName = strings.TrimSpace(provider.DisplayName)
	if log.ProviderName == "" {
		log.ProviderName = strings.TrimSpace(provider.VendorName)
	}
	return log
}

func probeRequest(provider *domains.VendorMeta, modelName string) (string, string, []byte, error) {
	if provider == nil {
		return "", "", nil, errors.New("provider is required")
	}
	providerType := strings.TrimSpace(provider.Type)
	if providerType == "" {
		providerType = constants.ProviderTypeOpenAI
	}
	upstreamModel := ProviderServiceApp.MapModel(provider, modelName)
	switch providerType {
	case constants.ProviderTypeAnthropic:
		body, _ := json.Marshal(map[string]any{
			"model":      upstreamModel,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		})
		return http.MethodPost, "/v1/messages", body, nil
	case constants.ProviderTypeGemini:
		body, _ := json.Marshal(map[string]any{
			"contents": []map[string]any{
				{"parts": []map[string]string{{"text": "ping"}}},
			},
			"generationConfig": map[string]any{"maxOutputTokens": 1},
		})
		return http.MethodPost, "/v1beta/models/" + url.PathEscape(upstreamModel) + ":generateContent", body, nil
	default:
		body, _ := json.Marshal(map[string]any{
			"model":      upstreamModel,
			"max_tokens": 1,
			"stream":     false,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		})
		return http.MethodPost, "/v1/chat/completions", body, nil
	}
}

func probeURL(provider *domains.VendorMeta, upstreamPath string) string {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(provider.Type)
	}
	targetURL := baseURL + upstreamPath
	if provider.Type == constants.ProviderTypeGemini {
		targetURL = attachGeminiKey(targetURL, provider.Key, "")
	}
	return applyParamOverride(targetURL, provider.ParamOverride)
}

func probeBodySnippet(statusCode int, body []byte) string {
	content := strings.TrimSpace(string(body))
	if len(content) > 1000 {
		content = content[:1000]
	}
	if content == "" {
		return fmt.Sprintf("upstream returned %d", statusCode)
	}
	return fmt.Sprintf("upstream returned %d: %s", statusCode, content)
}
