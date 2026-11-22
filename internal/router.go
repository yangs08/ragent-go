package internal

import (
	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/controller"
	"ragent-go/internal/middleware"
)

// RegisterRoutes binds HTTP endpoints to an already configured Gin engine.
func RegisterRoutes(r *gin.Engine) {
	{
		r.GET("/healthz", controller.Health.Check)
		r.HEAD("/healthz", controller.Health.Head)
	}
	// Database-disabled development mode exposes health endpoints only.
	if bootstrap.DB == nil {
		return
	}
	{
		r.POST("/auth/login", middleware.LoginLimit(), controller.Auth.Login)
	}
	protected := r.Group("", middleware.RequireLogin())
	{
		protected.GET("/user/me", controller.Auth.Me)
		protected.POST("/auth/logout", controller.Auth.Logout)
	}
}
