package middleware

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
)

func RequestLog() gin.HandlerFunc {
	logger := bootstrap.Logger
	return func(c *gin.Context) {
		started := time.Now()
		requestID := rand.Text()
		c.Header("X-Request-ID", requestID)
		c.Next()
		logger.InfoContext(c.Request.Context(), "http request completed",
			"request_id", requestID, "method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
	}
}

// RequestTimeout 为整个 HTTP 调用链设置一次 deadline，Service/DAO 沿用该 ctx。
// 这是协作式取消；不启动后台 handler，也不与 handler 并发写响应。
func RequestTimeout() gin.HandlerFunc {
	timeout := bootstrap.Config.Server.RequestTimeout
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
