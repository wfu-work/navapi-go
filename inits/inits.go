package inits

import (
	"context"
	_ "embed"
	"fmt"
	"navapi-go/domains"
	"navapi-go/scheduleds"
	"navapi-go/utils"
	"navapi-go/webs"
	"os"
	"time"

	"navapi-go/routers"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/wfu-work/nav-common-go-lib/global"
	"github.com/wfu-work/nav-common-go-lib/inits"
	commonScheduleds "github.com/wfu-work/nav-common-go-lib/scheduleds"
	"go.uber.org/zap"
)

//go:embed config.yaml
var defaultConfig []byte

const defaultUsageLogRetentionDays int64 = 90

func Init() {
	if err := utils.NewDefaultConfigManager(defaultConfig).Ensure(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "prepare config failed: %v\n", err)
		os.Exit(1)
	}
	cleanupRuntimeConfig, err := utils.PrepareDatabaseEnvConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "prepare database environment config failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanupRuntimeConfig()
	sysInit := inits.SysInit{}
	sysInit.OnTableInit(func() {
		domains.RegisterTables()
	})
	sysInit.OnRouterInit(func(publicGroup *gin.RouterGroup, privateGroup *gin.RouterGroup) {
		routers.RouterGroupApp.InitRouters(publicGroup, privateGroup)
	})
	sysInit.OnWebInit(func(router *gin.Engine) {
		_ = webs.InitStatic(router)
	})
	sysInit.OnOtherInit(func() {
		if err := configurePostgresRuntime(); err != nil {
			global.NAV_LOG.Error("configure postgres runtime failed", zap.Error(err))
			os.Exit(1)
		}
		_ = services.OptionServiceApp.Load()
		if err := services.PostgresServiceApp.Ensure(global.NAV_DB); err != nil {
			global.NAV_LOG.Error("ensure postgres migrations failed", zap.Error(err))
			os.Exit(1)
		}
		services.MessageTemplateServiceApp.SeedDefaults()
		if err := services.ModelServiceApp.EnsureDefaultGroup(); err != nil {
			global.NAV_LOG.Error("ensure default model group failed", zap.Error(err))
			os.Exit(1)
		}
		if err := services.PermissionSeedServiceApp.Ensure(); err != nil {
			global.NAV_LOG.Error("ensure navapi permissions failed", zap.Error(err))
			os.Exit(1)
		}
	})
	sysInit.OnScheInit(func(timers commonScheduleds.Timer, options []cron.Option) {
		scheduleds.Init(timers, options)
	})
	sysInit.OnClearInit(func() []commonScheduleds.ClearDB {
		retentionDays := min(services.OptionServiceApp.Int64("usage.log_retention_days", defaultUsageLogRetentionDays), 3650)
		var clearDBs []commonScheduleds.ClearDB
		if retentionDays > 0 {
			clearDBs = append(clearDBs, commonScheduleds.ClearDB{
				TableName:    (domains.UsageLog{}).TableName(),
				CompareField: "create_time",
				Interval:     fmt.Sprintf("%dh", retentionDays*24),
			})
		}
		return clearDBs
	})
	sysInit.Init()
}

func configurePostgresRuntime() error {
	if global.NAV_DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if dialect := global.NAV_DB.Dialector.Name(); dialect != "postgres" {
		return fmt.Errorf("postgres database is required, got %q", dialect)
	}
	sqlDB, err := global.NAV_DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}
