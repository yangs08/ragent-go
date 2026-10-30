package internal

import (
	"github.com/gin-gonic/gin"

	"ragent-go/internal/controller"
	"ragent-go/internal/middleware"
	"ragent-go/internal/service"
)

// RegisterRoutes binds HTTP endpoints to an already configured Gin engine.
func RegisterRoutes(r *gin.Engine) {
	health := controller.HealthController{}
	{
		r.GET("/healthz", health.Check)
		r.HEAD("/healthz", health.Head)
	}
	// Database-disabled development mode exposes health endpoints only.
	if service.Auth == nil {
		return
	}
	api := controller.AuthController{}
	{
		r.POST("/auth/login", middleware.LoginLimit(), api.Login)
	}
	protected := r.Group("", middleware.RequireLogin())
	{
		protected.GET("/user/me", api.Me)
		protected.POST("/auth/logout", api.Logout)
	}
}
