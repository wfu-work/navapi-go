package scheduleds

import (
	"context"

	"navapi-go/services"

	"github.com/robfig/cron/v3"
	"github.com/wfu-work/nav-common-go-lib/global"
	commonScheduleds "github.com/wfu-work/nav-common-go-lib/scheduleds"
	"go.uber.org/zap"
)

func Init(timers commonScheduleds.Timer, options []cron.Option) {
	_, _ = timers.AddTaskByFunc("navapi", "@every 1m", func() {
		_ = services.OptionServiceApp.Load()
	}, "refresh_navapi_options", options...)
	_, _ = timers.AddTaskByFunc("navapi", "@every 1m", func() {
		if !services.OptionServiceApp.Bool("provider.balance_refresh_enabled", true) {
			return
		}
		summary, err := services.ProviderServiceApp.RefreshDueBalances(context.Background())
		if err != nil {
			global.NAV_LOG.Error("refresh provider balances failed", zap.Error(err))
			return
		}
		if summary.Due > 0 {
			global.NAV_LOG.Info(
				"refresh provider balances completed",
				zap.Int("due", summary.Due),
				zap.Int("succeeded", summary.Succeeded),
				zap.Int("failed", summary.Failed),
			)
		}
	}, "refresh_provider_balances", options...)
}
