package middleware

import (
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
