package services

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
)

const GatewayVersion = "v0.1.0"

const (
	serviceStatusRecentRequestCount         = 60
	serviceStatusCacheTTL                   = 30 * time.Second
	serviceStatusWarningFirstResponseTimeMs = int64(5000)
	serviceStatusWarningSuccessRate         = 0.99
	serviceStatusCriticalSuccessRate        = 0.95
)

var gatewayStartedAt = time.Now()
var GatewayServiceApp = new(GatewayService)
var gatewayStatusCache = struct {
	sync.RWMutex
	Mode      string
	ExpiresAt time.Time
	Status    PublicServiceStatus
}{}

type GatewayService struct{}

type GatewayHealth struct {
	Status           string `json:"status"`
	Version          string `json:"version"`
	Mode             string `json:"mode"`
	StartedAt        int64  `json:"startedAt"`
	UptimeSeconds    int64  `json:"uptimeSeconds"`
	DatabaseStatus   string `json:"databaseStatus"`
	QueueSize        int64  `json:"queueSize"`
	InflightRequests int64  `json:"inflightRequests"`
	Goroutines       int    `json:"goroutines"`
	MemoryAllocBytes uint64 `json:"memoryAllocBytes"`
}

type PublicServiceStatus struct {
	Status             string                     `json:"status"`
	StatusLabel        string                     `json:"statusLabel"`
	UpdatedAt          int64                      `json:"updatedAt"`
	RecentRequestCount int                        `json:"recentRequestCount"`
	Health             GatewayHealth              `json:"health"`
	Summary            PublicServiceStatusSummary `json:"summary"`
	Models             []PublicModelStatus        `json:"models"`
}

type PublicServiceStatusSummary struct {
	EnabledModels          int     `json:"enabledModels"`
	ActiveModels           int     `json:"activeModels"`
	TotalRequests          int64   `json:"totalRequests"`
	SuccessRequests        int64   `json:"successRequests"`
	ErrorRequests          int64   `json:"errorRequests"`
	AvgFirstResponseTimeMs int64   `json:"avgFirstResponseTimeMs"`
	SuccessRate            float64 `json:"successRate"`
}

type PublicModelStatus struct {
	ModelName           string                     `json:"modelName"`
	DisplayName         string                     `json:"displayName,omitempty"`
	Status              string                     `json:"status"`
	StatusLabel         string                     `json:"statusLabel"`
	LastRequestAt       int64                      `json:"lastRequestAt,omitempty"`
	FirstResponseTimeMs int64                      `json:"firstResponseTimeMs"`
	Requests            int64                      `json:"requests"`
	SuccessRequests     int64                      `json:"successRequests"`
	ErrorRequests       int64                      `json:"errorRequests"`
	SuccessRate         float64                    `json:"successRate"`
	Segments            []PublicModelStatusSegment `json:"segments"`
	latencyTotalMs      int64
	latencySamples      int64
}

type PublicModelStatusSegment struct {
	Tone                string  `json:"tone"`
	Label               string  `json:"label"`
	StartTime           int64   `json:"startTime"`
	EndTime             int64   `json:"endTime"`
	Requests            int64   `json:"requests"`
	Success             int64   `json:"success"`
	Errors              int64   `json:"errors"`
	FirstResponseTimeMs int64   `json:"firstResponseTimeMs"`
	LatencySamples      int64   `json:"latencySamples"`
	SuccessRate         float64 `json:"successRate"`
}

type serviceModelAggregate struct {
	modelName       string
	displayName     string
	lastRequestAt   int64
	requests        int64
	successRequests int64
	errorRequests   int64
	latencyTotalMs  int64
	latencySamples  int64
	requestsData    []serviceRequestAggregate
}

type serviceRequestAggregate struct {
	eventTime       int64
	success         bool
	firstResponseMs int64
}

type serviceUsageRequestRow struct {
	ID                  uint   `gorm:"column:id"`
	ModelName           string `gorm:"column:model_name"`
	Status              string `gorm:"column:status"`
	FirstResponseTimeMs int64  `gorm:"column:first_response_time_ms"`
	CreateTime          int64  `gorm:"column:create_time"`
	LastSeenTime        int64  `gorm:"column:last_seen_time"`
	RepeatCount         int64  `gorm:"column:repeat_count"`
	RequestRank         int64  `gorm:"column:request_rank"`
}

func (s GatewayService) Health(mode string) GatewayHealth {
	databaseStatus := databaseHealthStatus()
	status := "running"
	if databaseStatus != "ok" {
		status = "degraded"
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return GatewayHealth{
		Status:           status,
		Version:          GatewayVersion,
		Mode:             mode,
		StartedAt:        gatewayStartedAt.UnixMilli(),
		UptimeSeconds:    int64(time.Since(gatewayStartedAt).Seconds()),
		DatabaseStatus:   databaseStatus,
		QueueSize:        0,
		InflightRequests: 0,
		Goroutines:       runtime.NumGoroutine(),
		MemoryAllocBytes: mem.Alloc,
	}
}

func (s GatewayService) PublicStatus(mode string) (PublicServiceStatus, error) {
	now := time.Now()
	gatewayStatusCache.RLock()
	if gatewayStatusCache.Mode == mode && now.Before(gatewayStatusCache.ExpiresAt) {
		status := gatewayStatusCache.Status
		gatewayStatusCache.RUnlock()
		return status, nil
	}
	gatewayStatusCache.RUnlock()

	status, err := s.publicStatus(mode, now)
	if err != nil {
		return PublicServiceStatus{}, err
	}
	gatewayStatusCache.Lock()
	gatewayStatusCache.Mode = mode
	gatewayStatusCache.ExpiresAt = now.Add(serviceStatusCacheTTL)
	gatewayStatusCache.Status = status
	gatewayStatusCache.Unlock()
	return status, nil
}

func (s GatewayService) publicStatus(mode string, now time.Time) (PublicServiceStatus, error) {
	health := s.Health(mode)
	status := PublicServiceStatus{
		Status:             "success",
		StatusLabel:        "正常",
		UpdatedAt:          now.UnixMilli(),
		RecentRequestCount: serviceStatusRecentRequestCount,
		Health:             health,
	}
	if health.DatabaseStatus != "ok" {
		status.Status = "danger"
		status.StatusLabel = "异常"
		return status, nil
	}

	models, err := ModelServiceApp.PublicListMeta()
	if err != nil {
		return PublicServiceStatus{}, err
	}
	rows, err := s.recentUsageRequests(publicModelNames(models))
	if err != nil {
		return PublicServiceStatus{}, err
	}
	status.Summary.EnabledModels = len(models)
	status.Models = buildPublicModelStatuses(models, rows)
	status.Summary = summarizePublicServiceStatus(status.Summary, status.Models)
	status.Status = publicServiceOverallTone(status.Health, status.Summary, status.Models)
	status.StatusLabel = publicServiceStatusLabel(status.Status, true)
	return status, nil
}

// recentUsageRequests returns the latest request log rows for each public model.
// A usage log can represent several requests through repeat_count, so callers
// cap the expanded rows at serviceStatusRecentRequestCount when aggregating.
func (s GatewayService) recentUsageRequests(modelNames []string) ([]serviceUsageRequestRow, error) {
	if len(modelNames) == 0 {
		return nil, nil
	}
	requestRankExpr := "ROW_NUMBER() OVER (PARTITION BY model_name ORDER BY GREATEST(create_time, COALESCE(NULLIF(last_seen_time, 0), create_time)) DESC, id DESC)"
	ranked := global.NAV_DB.
		Model(&domains.UsageLog{}).
		Select(fmt.Sprintf("id, model_name, status, first_response_time_ms, create_time, last_seen_time, repeat_count, %s as request_rank", requestRankExpr)).
		Where("model_name IN ?", modelNames).
		Where("source = ?", domains.UsageLogSourceUser)
	var rows []serviceUsageRequestRow
	err := global.NAV_DB.
		Table("(?) AS recent_usage", ranked).
		Select("id, model_name, status, first_response_time_ms, create_time, last_seen_time, repeat_count, request_rank").
		Where("request_rank <= ?", serviceStatusRecentRequestCount).
		Order("model_name ASC, request_rank ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func publicModelNames(models []domains.ModelMeta) []string {
	seen := make(map[string]struct{}, len(models))
	modelNames := make([]string, 0, len(models))
	for _, model := range models {
		modelName := strings.TrimSpace(model.ModelName)
		if modelName == "" {
			continue
		}
		if _, exists := seen[modelName]; exists {
			continue
		}
		seen[modelName] = struct{}{}
		modelNames = append(modelNames, modelName)
	}
	return modelNames
}

func databaseHealthStatus() string {
	if global.NAV_DB == nil {
		return "unavailable"
	}
	sqlDB, err := global.NAV_DB.DB()
	if err != nil {
		return "error"
	}
	if err := sqlDB.Ping(); err != nil {
		return "error"
	}
	return "ok"
}

func buildPublicModelStatuses(models []domains.ModelMeta, rows []serviceUsageRequestRow) []PublicModelStatus {
	aggregates := make(map[string]*serviceModelAggregate, len(models))
	order := make([]string, 0, len(models))
	for _, model := range models {
		modelName := strings.TrimSpace(model.ModelName)
		if modelName == "" {
			continue
		}
		if _, ok := aggregates[modelName]; ok {
			continue
		}
		aggregates[modelName] = newServiceModelAggregate(modelName, strings.TrimSpace(model.DisplayName))
		order = append(order, modelName)
	}
	for _, row := range rows {
		modelName := strings.TrimSpace(row.ModelName)
		if modelName == "" {
			continue
		}
		aggregate := aggregates[modelName]
		if aggregate == nil {
			continue
		}
		aggregate.applyRequest(row)
	}
	out := make([]PublicModelStatus, 0, len(order))
	for _, modelName := range order {
		out = append(out, aggregates[modelName].toPublicStatus())
	}
	return out
}

func newServiceModelAggregate(modelName string, displayName string) *serviceModelAggregate {
	return &serviceModelAggregate{
		modelName:    modelName,
		displayName:  displayName,
		requestsData: make([]serviceRequestAggregate, 0, serviceStatusRecentRequestCount),
	}
}

func (a *serviceModelAggregate) applyRequest(row serviceUsageRequestRow) {
	remaining := serviceStatusRecentRequestCount - len(a.requestsData)
	if remaining <= 0 {
		return
	}
	weight := row.RepeatCount
	if weight <= 0 {
		weight = 1
	}
	if weight > int64(remaining) {
		weight = int64(remaining)
	}
	eventTime := row.CreateTime
	if row.LastSeenTime > eventTime {
		eventTime = row.LastSeenTime
	}
	success := strings.EqualFold(strings.TrimSpace(row.Status), "success")
	for index := int64(0); index < weight; index++ {
		a.requests++
		if success {
			a.successRequests++
		} else {
			a.errorRequests++
		}
		if row.FirstResponseTimeMs > 0 {
			a.latencyTotalMs += row.FirstResponseTimeMs
			a.latencySamples++
		}
		a.requestsData = append(a.requestsData, serviceRequestAggregate{
			eventTime:       eventTime,
			success:         success,
			firstResponseMs: row.FirstResponseTimeMs,
		})
		if eventTime > a.lastRequestAt {
			a.lastRequestAt = eventTime
		}
	}
}

func (a *serviceModelAggregate) toPublicStatus() PublicModelStatus {
	latency := avgLatency(a.latencyTotalMs, a.latencySamples)
	tone := publicServiceTone(a.requests, a.successRequests, a.errorRequests, latency)
	return PublicModelStatus{
		ModelName:           a.modelName,
		DisplayName:         a.displayName,
		Status:              tone,
		StatusLabel:         publicServiceStatusLabel(tone, false),
		LastRequestAt:       a.lastRequestAt,
		FirstResponseTimeMs: latency,
		Requests:            a.requests,
		SuccessRequests:     a.successRequests,
		ErrorRequests:       a.errorRequests,
		SuccessRate:         successRate(a.successRequests, a.requests),
		Segments:            a.segments(),
		latencyTotalMs:      a.latencyTotalMs,
		latencySamples:      a.latencySamples,
	}
}

func (a *serviceModelAggregate) segments() []PublicModelStatusSegment {
	segments := make([]PublicModelStatusSegment, 0, serviceStatusRecentRequestCount)
	idleCount := serviceStatusRecentRequestCount - len(a.requestsData)
	for index := 0; index < idleCount; index++ {
		segments = append(segments, PublicModelStatusSegment{
			Tone:        "idle",
			Label:       "暂无调用",
			SuccessRate: 0,
		})
	}
	requests := append([]serviceRequestAggregate(nil), a.requestsData...)
	sort.SliceStable(requests, func(left int, right int) bool {
		return requests[left].eventTime < requests[right].eventTime
	})
	for _, request := range requests {
		success := int64(0)
		errors := int64(1)
		if request.success {
			success = 1
			errors = 0
		}
		latency := request.firstResponseMs
		tone := publicServiceTone(1, success, errors, latency)
		segments = append(segments, PublicModelStatusSegment{
			Tone:                tone,
			Label:               publicServiceRequestSegmentLabel(request.eventTime, tone, latency),
			StartTime:           request.eventTime,
			EndTime:             request.eventTime,
			Requests:            1,
			Success:             success,
			Errors:              errors,
			FirstResponseTimeMs: latency,
			LatencySamples:      latencySampleCount(latency),
			SuccessRate:         successRate(success, 1),
		})
	}
	return segments
}

func summarizePublicServiceStatus(summary PublicServiceStatusSummary, models []PublicModelStatus) PublicServiceStatusSummary {
	latencyTotal := int64(0)
	latencySamples := int64(0)
	for _, model := range models {
		if model.Requests > 0 {
			summary.ActiveModels++
		}
		summary.TotalRequests += model.Requests
		summary.SuccessRequests += model.SuccessRequests
		summary.ErrorRequests += model.ErrorRequests
		latencyTotal += model.latencyTotalMs
		latencySamples += model.latencySamples
	}
	summary.AvgFirstResponseTimeMs = avgLatency(latencyTotal, latencySamples)
	summary.SuccessRate = successRate(summary.SuccessRequests, summary.TotalRequests)
	return summary
}

func publicServiceOverallTone(health GatewayHealth, summary PublicServiceStatusSummary, models []PublicModelStatus) string {
	if health.DatabaseStatus != "ok" {
		return "danger"
	}
	if summary.TotalRequests <= 0 {
		return "idle"
	}
	hasWarning := false
	for _, model := range models {
		if model.Status == "danger" {
			return "danger"
		}
		if model.Status == "warning" {
			hasWarning = true
		}
	}
	tone := publicServiceTone(summary.TotalRequests, summary.SuccessRequests, summary.ErrorRequests, summary.AvgFirstResponseTimeMs)
	if tone == "danger" {
		return tone
	}
	if tone == "warning" || hasWarning {
		return "warning"
	}
	return "success"
}

func publicServiceTone(requests int64, success int64, errors int64, firstResponseTimeMs int64) string {
	if requests <= 0 {
		return "idle"
	}
	rate := float64(success) / float64(requests)
	if errors > 0 && rate < serviceStatusCriticalSuccessRate {
		return "danger"
	}
	if errors > 0 || rate < serviceStatusWarningSuccessRate || firstResponseTimeMs >= serviceStatusWarningFirstResponseTimeMs {
		return "warning"
	}
	return "success"
}

func publicServiceStatusLabel(tone string, overall bool) string {
	switch tone {
	case "warning":
		if overall {
			return "部分波动"
		}
		return "存在波动"
	case "danger":
		return "异常"
	case "idle":
		return "暂无调用"
	default:
		return "正常"
	}
}

func publicServiceRequestSegmentLabel(eventTime int64, tone string, firstResponseTimeMs int64) string {
	timeLabel := "最近请求"
	if eventTime > 0 {
		timeLabel = time.UnixMilli(eventTime).Format("15:04:05")
	}
	return fmt.Sprintf("%s %s 1 次 首响 %dms", timeLabel, publicServiceStatusLabel(tone, false), firstResponseTimeMs)
}

func latencySampleCount(latency int64) int64 {
	if latency > 0 {
		return 1
	}
	return 0
}

func avgLatency(total int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return total / count
}

func successRate(success int64, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(success)*10000/float64(total)) / 100
}
