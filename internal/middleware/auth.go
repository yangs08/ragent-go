package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/common"
	"ragent-go/internal/service"
)

// RequireLogin verifies a raw Authorization token, then binds the trusted user.
// Future chat and conversation routes must use this same middleware.
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		if len(values) != 1 {
			common.NoAuth(c)
			return
		}
		user, err := service.Auth.Authenticate(c.Request.Context(), values[0])
		if err != nil {
			common.Error(c, err)
			return
		}
		c.Request = c.Request.WithContext(common.WithUser(c.Request.Context(), user))
		c.Next()
	}
}
func LoginLimit() gin.HandlerFunc {
	limiter := &loginLimiter{entries: make(map[string]loginEntry)}
	return limiter.allow
}

type loginEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}
type loginLimiter struct {
	mu          sync.Mutex
	entries     map[string]loginEntry
	nextCleanup time.Time
}

func (l *loginLimiter) allow(c *gin.Context) {
	cfg := bootstrap.Config.Auth
	now := time.Now()
	ip := c.ClientIP()
	l.mu.Lock()
	if !now.Before(l.nextCleanup) {
		for key, entry := range l.entries {
			if now.Sub(entry.lastSeen) > 2*cfg.LoginWindow {
				delete(l.entries, key)
			}
		}
		l.nextCleanup = now.Add(cfg.LoginWindow)
	}
	entry, ok := l.entries[ip]
	allowed := false
	if ok || len(l.entries) < 1024 {
		if !ok {
			entry.limiter = rate.NewLimiter(rate.Limit(float64(cfg.LoginAttempts)/cfg.LoginWindow.Seconds()), cfg.LoginAttempts)
		}
		entry.lastSeen = now
		l.entries[ip] = entry
		allowed = entry.limiter.AllowN(now, 1)
	}
	l.mu.Unlock()
	if !allowed {
		common.Fail(c, "登录尝试过于频繁，请稍后重试")
		return
	}
	c.Next()
}
