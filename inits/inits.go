package inits

import (
	_ "embed"
	"fmt"
	"navapi-go/domains"
	"navapi-go/utils"
	"os"

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

func Init() {
	if err := utils.NewDefaultConfigManager(defaultConfig).Ensure(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "prepare config failed: %v\n", err)
		os.Exit(1)
	}
	sysInit := inits.SysInit{}
	sysInit.OnTableInit(func() {
		domains.RegisterTables()
	})
	sysInit.OnRouterInit(func(publicGroup *gin.RouterGroup, privateGroup *gin.RouterGroup) {
		routers.RouterGroupApp.InitRouters(publicGroup, privateGroup)
	})
	sysInit.OnWebInit(func(engine *gin.Engine) {
		routers.RouterGroupApp.InitRelayRouter(engine)
	})
	sysInit.OnOtherInit(func() {
		_ = services.OptionServiceApp.Load()
		services.MessageTemplateServiceApp.SeedDefaults()
		if err := services.ModelServiceApp.EnsureDefaultGroup(); err != nil {
			global.NAV_LOG.Error("ensure default model group failed", zap.Error(err))
			os.Exit(1)
		}
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
