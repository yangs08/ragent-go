package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"ragent-go/internal/common"
	"ragent-go/internal/config"
	"ragent-go/internal/service"
)

// Result follows Ragent's JSON envelope. Health responses remain independent.
type Result struct {
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}

func writeResult(c *gin.Context, code, message string, data any) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/json")
	c.Status(http.StatusOK)
	_ = json.NewEncoder(c.Writer).Encode(Result{Code: code, Message: message, Data: data, RequestID: c.Writer.Header().Get("X-Request-ID")})
}
func authError(c *gin.Context, err error) {
	code := "A000001"
	message := "未登录或登录已过期"
	if errors.Is(err, service.ErrAuthUnavailable) {
		code = "B000001"
		message = "认证服务暂不可用"
	}
	writeResult(c, code, message, nil)
	c.Abort()
}

// RequireLogin verifies a raw Authorization token, then binds the trusted user.
// Future chat and conversation routes must use this same middleware.
func RequireLogin(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		if len(values) != 1 {
			authError(c, service.ErrUnauthenticated)
			return
		}
		user, err := auth.Authenticate(c.Request.Context(), values[0])
		if err != nil {
			authError(c, err)
			return
		}
		c.Request = c.Request.WithContext(common.WithUser(c.Request.Context(), user))
		c.Next()
	}
}
func registerAuth(router *gin.Engine, auth *service.AuthService, cfg config.AuthConfig) {
	limiter := &loginLimiter{entries: make(map[string]loginEntry), cfg: cfg}
	router.POST("/auth/login", limiter.allow, func(c *gin.Context) {
		media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || media != "application/json" {
			writeResult(c, "A000001", "需要 JSON 请求体", nil)
			return
		}
		var request struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeResult(c, "A000001", "登录参数不合法", nil)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeResult(c, "A000001", "登录参数不合法", nil)
			return
		}
		result, err := auth.Login(c.Request.Context(), request.Username, request.Password)
		if err != nil {
			if errors.Is(err, service.ErrCredentials) {
				writeResult(c, "A000001", "用户名或密码错误", nil)
			} else {
				authError(c, err)
			}
			return
		}
		writeResult(c, "0", "", result)
	})
	protected := router.Group("", RequireLogin(auth))
	protected.GET("/user/me", func(c *gin.Context) {
		user, err := common.RequireUser(c.Request.Context())
		if err != nil {
			authError(c, service.ErrUnauthenticated)
			return
		}
		writeResult(c, "0", "", user)
	})
	protected.POST("/auth/logout", func(c *gin.Context) {
		if err := auth.Logout(c.Request.Context(), c.GetHeader("Authorization")); err != nil {
			authError(c, err)
			return
		}
		writeResult(c, "0", "", nil)
	})
}

type loginEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}
type loginLimiter struct {
	mu          sync.Mutex
	entries     map[string]loginEntry
	cfg         config.AuthConfig
	nextCleanup time.Time
}

func (l *loginLimiter) allow(c *gin.Context) {
	now := time.Now()
	ip := c.ClientIP()
	l.mu.Lock()
	if !now.Before(l.nextCleanup) {
		for key, entry := range l.entries {
			if now.Sub(entry.lastSeen) > 2*l.cfg.LoginWindow {
				delete(l.entries, key)
			}
		}
		l.nextCleanup = now.Add(l.cfg.LoginWindow)
	}
	entry, ok := l.entries[ip]
	allowed := false
	if ok || len(l.entries) < 1024 {
		if !ok {
			entry.limiter = rate.NewLimiter(rate.Limit(float64(l.cfg.LoginAttempts)/l.cfg.LoginWindow.Seconds()), l.cfg.LoginAttempts)
		}
		entry.lastSeen = now
		l.entries[ip] = entry
		allowed = entry.limiter.AllowN(now, 1)
	}
	l.mu.Unlock()
	if !allowed {
		writeResult(c, "A000001", "登录尝试过于频繁，请稍后重试", nil)
		c.Abort()
		return
	}
	c.Next()
}
