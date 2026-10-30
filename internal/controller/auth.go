package controller

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/common"
	"ragent-go/internal/define"
	"ragent-go/internal/service"
)

type AuthController struct{}

func (a *AuthController) Login(c *gin.Context) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		common.Fail(c, "需要 JSON 请求体")
		return
	}
	var request define.LoginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		common.Fail(c, "登录参数不合法")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		common.Fail(c, "登录参数不合法")
		return
	}
	result, err := service.Auth.Login(c.Request.Context(), request.Username, request.Password)
	if err != nil {
		common.Error(c, err)
		return
	}
	common.Success(c, result)
}
func (a *AuthController) Me(c *gin.Context) {
	user, err := common.RequireUser(c.Request.Context())
	if err != nil {
		common.Error(c, err)
		return
	}
	common.Success(c, user)
}
func (a *AuthController) Logout(c *gin.Context) {
	if err := service.Auth.Logout(c.Request.Context(), c.GetHeader("Authorization")); err != nil {
		common.Error(c, err)
		return
	}
	common.Success(c, nil)
}
