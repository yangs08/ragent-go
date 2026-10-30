package middleware

import (
	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/common"
)

func Recovery() gin.HandlerFunc {
	logger := bootstrap.Logger
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		logger.ErrorContext(c.Request.Context(), "http handler panic recovered", "request_id", c.Writer.Header().Get("X-Request-ID"), "response_committed", c.Writer.Written())
		common.InternalError(c)
	})
}
