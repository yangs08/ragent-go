package common

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
)

func OutputSuccess(c *gin.Context, data any) {
	writeResponse(c, define.CodeSuccess, "", data)
}

// OutputError maps service errors to the existing HTTP 200 business envelope.
func OutputError(c *gin.Context, err error) {
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
		attrs := FailureAttrs(operation, err)
		attrs = append(attrs, "request_id", c.Writer.Header().Get("X-Request-ID"))
		bootstrap.Logger.ErrorContext(c.Request.Context(), "request failed", attrs...)
	}
	c.Abort()
	writeResponse(c, code, message, nil)
}
func writeResponse(c *gin.Context, code, message string, data any) {
	if c.Writer.Written() {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, define.Result{Code: code, Message: message, Data: data, RequestID: c.Writer.Header().Get("X-Request-ID")})
}
