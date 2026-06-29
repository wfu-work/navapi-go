package routers

import "github.com/gin-gonic/gin"

type OptionRouter struct{}

func (r OptionRouter) InitOptionRouter(privateGroup *gin.RouterGroup, publicGroup *gin.RouterGroup) {
	publicGroup.GET("register/settings", optionApi.RegisterSettings)

	group := privateGroup.Group("option")
	{
		group.GET("/list", optionApi.All)
		group.GET("/risk_control", optionApi.RiskControl)
		group.PUT("/risk_control", optionApi.SetRiskControl)
		group.GET("/register_settings", optionApi.RegisterSettings)
		group.PUT("/register_settings", optionApi.SetRegisterSettings)
		group.PUT("/", optionApi.Set)
		group.DELETE("/:key", optionApi.Delete)
	}
}
