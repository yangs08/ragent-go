package handler

import (
	"crypto/rand"
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"
)

func requestLog(logger *slog.Logger) gin.HandlerFunc {
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
