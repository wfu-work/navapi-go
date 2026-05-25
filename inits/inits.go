package inits

import (
	"os"

	"navapi-go/domains"
	"navapi-go/routers"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/wfu-work/nav-common-go-lib/global"
	"github.com/wfu-work/nav-common-go-lib/inits"
	commonScheduleds "github.com/wfu-work/nav-common-go-lib/scheduleds"
	"go.uber.org/zap"
)

func Init() {
	sysInit := inits.SysInit{}
	sysInit.OnTableInit(registerTables)
	sysInit.OnRouterInit(func(publicGroup *gin.RouterGroup, privateGroup *gin.RouterGroup) {
		routers.RouterGroupApp.InitRouters(publicGroup, privateGroup, nil)
	})
	sysInit.OnWebInit(func(engine *gin.Engine) {
		routers.RouterGroupApp.InitRelayRouter(engine)
	})
	sysInit.OnOtherInit(func() {
		_ = services.OptionServiceApp.Load()
	})
	sysInit.OnScheInit(func(timers commonScheduleds.Timer, options []cron.Option) {
		_, _ = timers.AddTaskByFunc("navapi", "@every 1m", func() {
			_ = services.OptionServiceApp.Load()
		}, "refresh_navapi_options", options...)
	})
	sysInit.OnClearInit(func() []commonScheduleds.ClearDB {
		return []commonScheduleds.ClearDB{}
	})
	sysInit.Init()
}

func registerTables() {
	db := global.NAV_DB
	if db == nil {
		return
	}
	if err := db.AutoMigrate(
		domains.Channel{},
		domains.ApiToken{},
		domains.UserQuota{},
		domains.UsageLog{},
		domains.ModelMeta{},
		domains.VendorMeta{},
		domains.Pricing{},
		domains.Option{},
		domains.Task{},
		domains.Redemption{},
		domains.QuotaDate{},
	); err != nil {
		global.NAV_LOG.Error("register navapi business tables failed", zap.Error(err))
		os.Exit(1)
	}
	global.NAV_LOG.Info("register navapi business tables success")
}
