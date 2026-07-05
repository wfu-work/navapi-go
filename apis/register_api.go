package apis

import (
	"navapi-go/services"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

type RegisterApi struct{}

type sendRegisterCodeRequest struct {
	Email string `json:"email"`
}

func (a RegisterApi) SendCode(c *gin.Context) {
	var req sendRegisterCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result, err := services.EmailServiceApp.SendRegisterCode(services.SendRegisterCodeInput{
		Email:    req.Email,
		ClientIP: c.ClientIP(),
	})
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

func (a RegisterApi) RegisterClient(c *gin.Context) {
	var req services.ClientRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	result, err := services.ClientRegisterServiceApp.Register(req)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}
