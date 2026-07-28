package scheduleds

import (
	"navapi-go/services"

	"github.com/robfig/cron/v3"
	commonScheduleds "github.com/wfu-work/nav-common-go-lib/scheduleds"
)

func Init(timers commonScheduleds.Timer, options []cron.Option) {
	_, _ = timers.AddTaskByFunc("navapi", "@every 1m", func() {
		_ = services.OptionServiceApp.Load()
	}, "refresh_navapi_options", options...)
}
