package services

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
)

const GatewayVersion = "v0.1.0"

const (
	serviceStatusSegmentCount        = 24
	serviceStatusWindow              = 24 * time.Hour
	serviceStatusCacheTTL            = 30 * time.Second
	serviceStatusWarningLatencyMs    = int64(3000)
	serviceStatusWarningSuccessRate  = 0.99
	serviceStatusCriticalSuccessRate = 0.95
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
	Status        string                     `json:"status"`
	StatusLabel   string                     `json:"statusLabel"`
	UpdatedAt     int64                      `json:"updatedAt"`
	WindowMinutes int                        `json:"windowMinutes"`
	Health        GatewayHealth              `json:"health"`
	Summary       PublicServiceStatusSummary `json:"summary"`
	Models        []PublicModelStatus        `json:"models"`
}

type PublicServiceStatusSummary struct {
	EnabledModels   int     `json:"enabledModels"`
	ActiveModels    int     `json:"activeModels"`
	TotalRequests   int64   `json:"totalRequests"`
	SuccessRequests int64   `json:"successRequests"`
	ErrorRequests   int64   `json:"errorRequests"`
	AvgLatencyMs    int64   `json:"avgLatencyMs"`
	SuccessRate     float64 `json:"successRate"`
}

type PublicModelStatus struct {
	ModelName       string                     `json:"modelName"`
	DisplayName     string                     `json:"displayName,omitempty"`
	Status          string                     `json:"status"`
	StatusLabel     string                     `json:"statusLabel"`
	LastCheckedAt   int64                      `json:"lastCheckedAt,omitempty"`
	LatencyMs       int64                      `json:"latencyMs"`
	Requests        int64                      `json:"requests"`
	SuccessRequests int64                      `json:"successRequests"`
	ErrorRequests   int64                      `json:"errorRequests"`
	SuccessRate     float64                    `json:"successRate"`
	Segments        []PublicModelStatusSegment `json:"segments"`
}

type PublicModelStatusSegment struct {
	Tone        string  `json:"tone"`
	Label       string  `json:"label"`
	StartTime   int64   `json:"startTime"`
	EndTime     int64   `json:"endTime"`
	Requests    int64   `json:"requests"`
	Success     int64   `json:"success"`
	Errors      int64   `json:"errors"`
	LatencyMs   int64   `json:"latencyMs"`
	SuccessRate float64 `json:"successRate"`
}

type serviceModelAggregate struct {
	modelName       string
	displayName     string
	lastCheckedAt   int64
	requests        int64
	successRequests int64
	errorRequests   int64
	latencyTotalMs  int64
	buckets         []serviceBucketAggregate
}

type serviceBucketAggregate struct {
	requests       int64
	success        int64
	errors         int64
	latencyTotalMs int64
}

type serviceUsageBucketRow struct {
	BucketIndex     int    `gorm:"column:bucket_index"`
	ModelName       string `gorm:"column:model_name"`
	Requests        int64  `gorm:"column:requests"`
	SuccessRequests int64  `gorm:"column:success_requests"`
	ErrorRequests   int64  `gorm:"column:error_requests"`
	LatencyTotalMs  int64  `gorm:"column:latency_total_ms"`
	LastCheckedAt   int64  `gorm:"column:last_checked_at"`
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
	start := now.Add(-serviceStatusWindow)
	health := s.Health(mode)
	status := PublicServiceStatus{
		Status:        "success",
		StatusLabel:   "正常",
		UpdatedAt:     now.UnixMilli(),
		WindowMinutes: int(serviceStatusWindow / time.Minute),
		Health:        health,
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
	rows, err := s.recentUsageBuckets(start, now)
	if err != nil {
		return PublicServiceStatus{}, err
	}
	status.Summary.EnabledModels = len(models)
	status.Models = buildPublicModelStatuses(models, rows, start, now)
	status.Summary = summarizePublicServiceStatus(status.Summary, status.Models)
	status.Status = publicServiceOverallTone(status.Health, status.Summary, status.Models)
	status.StatusLabel = publicServiceStatusLabel(status.Status, true)
	return status, nil
}

func resetGatewayStatusCache() {
	gatewayStatusCache.Lock()
	gatewayStatusCache.Mode = ""
	gatewayStatusCache.ExpiresAt = time.Time{}
	gatewayStatusCache.Status = PublicServiceStatus{}
	gatewayStatusCache.Unlock()
}

func (s GatewayService) recentUsageBuckets(start time.Time, end time.Time) ([]serviceUsageBucketRow, error) {
	if !end.After(start) {
		return nil, nil
	}
	span := end.Sub(start) / time.Duration(serviceStatusSegmentCount)
	if span <= 0 {
		return nil, nil
	}
	startMS := start.UnixMilli()
	endExclusive := end.UnixMilli() + 1
	spanMS := int64(span / time.Millisecond)
	if spanMS <= 0 {
		spanMS = 1
	}
	bucketExpr := serviceStatusBucketExprSQL(global.NAV_DB.Dialector.Name(), startMS, spanMS)
	selectSQL := fmt.Sprintf(`
		%s as bucket_index,
		model_name,
		COUNT(*) as requests,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success_requests,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 0 ELSE 1 END), 0) as error_requests,
		COALESCE(SUM(CASE WHEN first_response_time_ms > 0 THEN first_response_time_ms ELSE use_time_ms END), 0) as latency_total_ms,
		COALESCE(MAX(create_time), 0) as last_checked_at
	`, bucketExpr)
	var rows []serviceUsageBucketRow
	err := global.NAV_DB.
		Model(&domains.UsageLog{}).
		Select(selectSQL).
		Where("create_time >= ? AND create_time < ?", startMS, endExclusive).
		Group(bucketExpr + ", model_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func serviceStatusBucketExprSQL(dialect string, startMS int64, spanMS int64) string {
	if spanMS <= 0 {
		spanMS = 1
	}
	lastBucket := serviceStatusSegmentCount - 1
	switch dialect {
	case "mysql":
		return fmt.Sprintf("LEAST(FLOOR((create_time - %d) / %d), %d)", startMS, spanMS, lastBucket)
	case "postgres":
		return fmt.Sprintf("LEAST(FLOOR((create_time - %d)::numeric / %d), %d)", startMS, spanMS, lastBucket)
	default:
		return fmt.Sprintf("MIN(CAST((create_time - %d) / %d AS INTEGER), %d)", startMS, spanMS, lastBucket)
	}
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

func buildPublicModelStatuses(models []domains.ModelMeta, rows []serviceUsageBucketRow, start time.Time, end time.Time) []PublicModelStatus {
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
			aggregate = newServiceModelAggregate(modelName, "")
			aggregates[modelName] = aggregate
			order = append(order, modelName)
		}
		aggregate.applyBucket(row)
	}
	out := make([]PublicModelStatus, 0, len(order))
	for _, modelName := range order {
		out = append(out, aggregates[modelName].toPublicStatus(start, end))
	}
	return out
}

func newServiceModelAggregate(modelName string, displayName string) *serviceModelAggregate {
	return &serviceModelAggregate{
		modelName:   modelName,
		displayName: displayName,
		buckets:     make([]serviceBucketAggregate, serviceStatusSegmentCount),
	}
}

func (a *serviceModelAggregate) applyBucket(row serviceUsageBucketRow) {
	a.requests += row.Requests
	a.successRequests += row.SuccessRequests
	a.errorRequests += row.ErrorRequests
	a.latencyTotalMs += row.LatencyTotalMs
	if row.LastCheckedAt > a.lastCheckedAt {
		a.lastCheckedAt = row.LastCheckedAt
	}
	if row.BucketIndex < 0 || row.BucketIndex >= len(a.buckets) {
		return
	}
	bucket := &a.buckets[row.BucketIndex]
	bucket.requests += row.Requests
	bucket.success += row.SuccessRequests
	bucket.errors += row.ErrorRequests
	bucket.latencyTotalMs += row.LatencyTotalMs
}

func (a *serviceModelAggregate) toPublicStatus(start time.Time, end time.Time) PublicModelStatus {
	latency := avgLatency(a.latencyTotalMs, a.requests)
	tone := publicServiceTone(a.requests, a.successRequests, a.errorRequests, latency)
	return PublicModelStatus{
		ModelName:       a.modelName,
		DisplayName:     a.displayName,
		Status:          tone,
		StatusLabel:     publicServiceStatusLabel(tone, false),
		LastCheckedAt:   a.lastCheckedAt,
		LatencyMs:       latency,
		Requests:        a.requests,
		SuccessRequests: a.successRequests,
		ErrorRequests:   a.errorRequests,
		SuccessRate:     successRate(a.successRequests, a.requests),
		Segments:        a.segments(start, end),
	}
}

func (a *serviceModelAggregate) segments(start time.Time, end time.Time) []PublicModelStatusSegment {
	segments := make([]PublicModelStatusSegment, 0, len(a.buckets))
	span := end.Sub(start) / time.Duration(serviceStatusSegmentCount)
	for index, bucket := range a.buckets {
		segmentStart := start.Add(time.Duration(index) * span)
		segmentEnd := segmentStart.Add(span)
		latency := avgLatency(bucket.latencyTotalMs, bucket.requests)
		tone := publicServiceTone(bucket.requests, bucket.success, bucket.errors, latency)
		segments = append(segments, PublicModelStatusSegment{
			Tone:        tone,
			Label:       publicServiceSegmentLabel(segmentStart, bucket, tone, latency),
			StartTime:   segmentStart.UnixMilli(),
			EndTime:     segmentEnd.UnixMilli(),
			Requests:    bucket.requests,
			Success:     bucket.success,
			Errors:      bucket.errors,
			LatencyMs:   latency,
			SuccessRate: successRate(bucket.success, bucket.requests),
		})
	}
	return segments
}

func summarizePublicServiceStatus(summary PublicServiceStatusSummary, models []PublicModelStatus) PublicServiceStatusSummary {
	latencyTotal := int64(0)
	for _, model := range models {
		if model.Requests > 0 {
			summary.ActiveModels++
		}
		summary.TotalRequests += model.Requests
		summary.SuccessRequests += model.SuccessRequests
		summary.ErrorRequests += model.ErrorRequests
		latencyTotal += model.LatencyMs * model.Requests
	}
	summary.AvgLatencyMs = avgLatency(latencyTotal, summary.TotalRequests)
	summary.SuccessRate = successRate(summary.SuccessRequests, summary.TotalRequests)
	return summary
}

func publicServiceOverallTone(health GatewayHealth, summary PublicServiceStatusSummary, models []PublicModelStatus) string {
	if health.DatabaseStatus != "ok" {
		return "danger"
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
	tone := publicServiceTone(summary.TotalRequests, summary.SuccessRequests, summary.ErrorRequests, summary.AvgLatencyMs)
	if tone == "danger" {
		return tone
	}
	if tone == "warning" || hasWarning {
		return "warning"
	}
	return "success"
}

func publicServiceTone(requests int64, success int64, errors int64, latencyMs int64) string {
	if requests <= 0 {
		return "idle"
	}
	rate := float64(success) / float64(requests)
	if errors > 0 && rate < serviceStatusCriticalSuccessRate {
		return "danger"
	}
	if errors > 0 || rate < serviceStatusWarningSuccessRate || latencyMs >= serviceStatusWarningLatencyMs {
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
		return "延迟偏高"
	case "danger":
		return "异常"
	case "idle":
		return "暂无调用"
	default:
		return "正常"
	}
}

func publicServiceSegmentLabel(start time.Time, bucket serviceBucketAggregate, tone string, latencyMs int64) string {
	timeLabel := start.Format("15:04")
	if bucket.requests <= 0 {
		return timeLabel + " 暂无调用"
	}
	return timeLabel + " " + publicServiceStatusLabel(tone, false) + " " + strconv.FormatInt(bucket.requests, 10) + " 次 " + strconv.FormatInt(latencyMs, 10) + "ms"
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
