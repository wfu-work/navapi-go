package services

import (
	"runtime"
	"time"

	"github.com/wfu-work/nav-common-go-lib/global"
)

const GatewayVersion = "v0.1.0"

var gatewayStartedAt = time.Now()
var GatewayServiceApp = GatewayService{}

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
