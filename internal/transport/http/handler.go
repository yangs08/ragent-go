package httptransport

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewHandler returns a Gin router for the application's existing http.Server.
func NewHandler(logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	// Only explicitly configured reverse proxies may supply trusted client IPs.
	_ = router.SetTrustedProxies(nil)
	router.Use(requestLog(logger))
	router.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		logger.ErrorContext(c.Request.Context(), "http handler panic recovered",
			"request_id", c.Writer.Header().Get("X-Request-ID"), "response_committed", c.Writer.Written())
		c.Abort()
		// Streaming output cannot be replaced once headers or body have been sent.
		if !c.Writer.Written() {
			c.Header("Content-Type", "application/json")
			c.Status(http.StatusInternalServerError)
			_ = json.NewEncoder(c.Writer).Encode(map[string]string{"error": "internal_server_error"})
		}
	}))
	router.GET("/healthz", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		_ = json.NewEncoder(c.Writer).Encode(map[string]string{"status": "ok"})
	})
	router.HEAD("/healthz", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusOK)
	})
	return router
}
