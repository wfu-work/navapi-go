package apis

import (
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type OptionApi struct{}

type optionRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

func (a OptionApi) All(c *gin.Context) {
	options, err := services.OptionServiceApp.All()
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(options, c)
}

func (a OptionApi) Set(c *gin.Context) {
	var req optionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := services.OptionServiceApp.Set(req.Key, req.Value); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}

func (a OptionApi) Delete(c *gin.Context) {
	if err := services.OptionServiceApp.Delete(c.Param("key")); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(true, c)
}
