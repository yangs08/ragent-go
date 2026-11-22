package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
)

func Recovery() gin.HandlerFunc {
	logger := bootstrap.Logger
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		logger.ErrorContext(c.Request.Context(), "http handler panic recovered", "request_id", c.Writer.Header().Get("X-Request-ID"), "response_committed", c.Writer.Written())
		c.Abort()
		// 流式或已写出的响应不能再追加错误 JSON。
		if c.Writer.Written() {
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusInternalServerError, define.Result{
			Code: define.CodeServerError, Message: "internal_server_error",
			RequestID: c.Writer.Header().Get("X-Request-ID"),
		})
	})
}
