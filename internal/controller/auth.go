package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/common"
	"ragent-go/internal/define"
	"ragent-go/internal/service"
)

var Auth = new(authController)

type authController struct{}

func (a *authController) Login(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var request define.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.OutputError(c, define.ClientError("请求参数不合法"))
		return
	}
	if !define.UsernamePattern.MatchString(request.Username) {
		common.OutputError(c, define.ClientError("用户名或密码错误"))
		return
	}
	result, err := service.Auth.Login(c.Request.Context(), request)
	if err != nil {
		common.OutputError(c, err)
		return
	}
	common.OutputSuccess(c, result)
}
func (a *authController) Me(c *gin.Context) {
	user, err := common.RequireUser(c.Request.Context())
	if err != nil {
		common.OutputError(c, err)
		return
	}
	common.OutputSuccess(c, user)
}
func (a *authController) Logout(c *gin.Context) {
	if err := service.Auth.Logout(c.Request.Context(), c.GetHeader("Authorization")); err != nil {
		common.OutputError(c, err)
		return
	}
	common.OutputSuccess(c, nil)
}
