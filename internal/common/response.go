package common

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
)

func Success(c *gin.Context, data any) {
	writeResponse(c, http.StatusOK, define.CodeSuccess, "", data)
}
func Fail(c *gin.Context, message string) {
	c.Abort()
	writeResponse(c, http.StatusOK, define.CodeClientError, message, nil)
}
func NoAuth(c *gin.Context) { Fail(c, "未登录或登录已过期") }

// Error maps service errors to the existing HTTP 200 business envelope.
func Error(c *gin.Context, err error) {
	code, message := define.CodeServerError, "系统暂不可用"
	var business *define.BusinessError
	if errors.As(err, &business) {
		code, message = business.Code, business.Message
	}
	if code == define.CodeServerError {
		operation := "http request"
		if business != nil && business.Operation != "" {
			operation = business.Operation
		}
		logger := bootstrap.Logger
		if logger == nil {
			logger = slog.Default()
		}
		attrs := FailureAttrs(operation, err)
		attrs = append(attrs, "request_id", c.Writer.Header().Get("X-Request-ID"))
		logger.ErrorContext(c.Request.Context(), "request failed", attrs...)
	}
	c.Abort()
	writeResponse(c, http.StatusOK, code, message, nil)
}
func InternalError(c *gin.Context) {
	c.Abort()
	if !c.Writer.Written() {
		writeResponse(c, http.StatusInternalServerError, define.CodeServerError, "internal_server_error", nil)
	}
}
func writeResponse(c *gin.Context, status int, code, message string, data any) {
	if c.Writer.Written() {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/json")
	c.Status(status)
	_ = json.NewEncoder(c.Writer).Encode(define.Result{Code: code, Message: message, Data: data, RequestID: c.Writer.Header().Get("X-Request-ID")})
}
