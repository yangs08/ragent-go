package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/common"
	"ragent-go/internal/define"
)

// 限制单个中间件实例保存的 IP 数量，避免不同来源持续占用内存。
const maxLoginIPs = 1024

// LoginLimit 按客户端 IP 限制登录尝试；每次注册创建独立状态，额度不跨进程共享。
func LoginLimit() gin.HandlerFunc {
	limiter := &loginLimiter{entries: make(map[string]loginEntry)}
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			common.OutputError(c, define.ClientError("登录尝试过于频繁，请稍后重试"))
			return
		}
		c.Next()
	}
}

type loginEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time // 包括被拒绝的尝试，持续访问的 IP 不因清理而重获额度。
}

// loginLimiter 只管理限流状态，不接收 HTTP context 或输出响应。
type loginLimiter struct {
	mu          sync.Mutex
	entries     map[string]loginEntry
	nextCleanup time.Time // 每个补充周期最多扫描一次，清理由请求触发。
}

func (l *loginLimiter) allow(ip string) bool {
	cfg := bootstrap.Config.Auth
	l.mu.Lock()
	defer l.mu.Unlock()
	// 获锁后取时间，避免并发请求按不同顺序获锁时回写较早的 lastSeen。
	now := time.Now()
	if !now.Before(l.nextCleanup) {
		// 空闲超过两个补充周期才回收；此时旧桶已可补满，重建不会额外增加额度。
		idleTimeout := 2 * cfg.LoginWindow
		for key, entry := range l.entries {
			if now.Sub(entry.lastSeen) > idleTimeout {
				delete(l.entries, key)
			}
		}
		l.nextCleanup = now.Add(cfg.LoginWindow)
	}
	entry, ok := l.entries[ip]
	if !ok {
		// 容量满时只拒绝新 IP，已有 IP 仍按自己的令牌桶判断。
		if len(l.entries) >= maxLoginIPs {
			return false
		}
		entry.limiter = rate.NewLimiter(rate.Limit(float64(cfg.LoginAttempts)/cfg.LoginWindow.Seconds()), cfg.LoginAttempts)
	}
	entry.lastSeen = now
	l.entries[ip] = entry
	return entry.limiter.AllowN(now, 1)
}
