package middleware

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
)

func TestLoginLimitsAreIsolatedPerInstance(t *testing.T) {
	old := bootstrap.Config.Auth
	t.Cleanup(func() { bootstrap.Config.Auth = old })
	bootstrap.Config.Auth = define.AuthConfig{LoginAttempts: 1, LoginWindow: time.Hour}
	newRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(LoginLimit())
		r.GET("/", func(c *gin.Context) { c.String(200, "accepted") })
		return r
	}
	restricted, other := newRouter(), newRouter()
	request := func(r *gin.Engine) string {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		r.ServeHTTP(w, req)
		return w.Body.String()
	}
	if request(restricted) != "accepted" || !strings.Contains(request(restricted), "登录尝试过于频繁") {
		t.Fatal("first instance did not enforce its configured budget")
	}
	if request(other) != "accepted" {
		t.Fatal("first instance affected another instance")
	}
	if !strings.Contains(request(other), "登录尝试过于频繁") {
		t.Fatal("second instance ignored its own budget")
	}
}

func TestLoginLimitConcurrentRequestsShareIPBudget(t *testing.T) {
	old := bootstrap.Config.Auth
	t.Cleanup(func() { bootstrap.Config.Auth = old })
	bootstrap.Config.Auth = define.AuthConfig{LoginAttempts: 7, LoginWindow: time.Hour}
	limiter := &loginLimiter{entries: make(map[string]loginEntry)}
	var allowed atomic.Int32
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 100; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			if limiter.allow("127.0.0.1") {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	workers.Wait()
	if got := allowed.Load(); got != 7 {
		t.Fatalf("allowed %d requests, want 7", got)
	}
	if !limiter.allow("127.0.0.2") {
		t.Fatal("one IP exhausted another IP's budget")
	}
}

func TestLoginLimitCapacityAndIdleCleanup(t *testing.T) {
	old := bootstrap.Config.Auth
	t.Cleanup(func() { bootstrap.Config.Auth = old })
	bootstrap.Config.Auth = define.AuthConfig{LoginAttempts: 1, LoginWindow: time.Hour}
	now := time.Now()
	limiter := &loginLimiter{entries: make(map[string]loginEntry), nextCleanup: now.Add(time.Hour)}
	for i := 0; i < maxLoginIPs; i++ {
		limiter.entries[strconv.Itoa(i)] = loginEntry{limiter: rate.NewLimiter(rate.Every(time.Hour), 1), lastSeen: now}
	}
	if limiter.allow("new") {
		t.Fatal("new IP exceeded capacity")
	}
	if !limiter.allow("0") {
		t.Fatal("capacity blocked an existing IP with available tokens")
	}
	if limiter.allow("0") {
		t.Fatal("existing IP exceeded budget")
	}

	stale := limiter.entries["1"]
	stale.lastSeen = now.Add(-3 * time.Hour)
	limiter.entries["1"] = stale
	if limiter.allow("new") {
		t.Fatal("cleanup ran before its scheduled time")
	}
	limiter.nextCleanup = now.Add(-time.Second)
	if !limiter.allow("new") {
		t.Fatal("idle entry did not release capacity")
	}
	if _, exists := limiter.entries["1"]; exists {
		t.Fatal("idle entry retained")
	}
	if len(limiter.entries) != maxLoginIPs {
		t.Fatal("cleanup removed active entries")
	}
	if limiter.allow("0") {
		t.Fatal("cleanup reset active IP's exhausted budget")
	}
	if !limiter.entries["0"].lastSeen.After(now) {
		t.Fatal("denied attempt did not refresh activity")
	}
}

func TestLoginLimitRefillsTokens(t *testing.T) {
	old := bootstrap.Config.Auth
	t.Cleanup(func() { bootstrap.Config.Auth = old })
	bootstrap.Config.Auth = define.AuthConfig{LoginAttempts: 1, LoginWindow: time.Hour}
	now := time.Now()
	bucket := rate.NewLimiter(rate.Every(time.Hour), 1)
	if !bucket.AllowN(now.Add(-2*time.Hour), 1) {
		t.Fatal("could not consume initial token")
	}
	limiter := &loginLimiter{
		entries:     map[string]loginEntry{"127.0.0.1": {limiter: bucket, lastSeen: now}},
		nextCleanup: now.Add(time.Hour),
	}
	if !limiter.allow("127.0.0.1") {
		t.Fatal("elapsed time did not replenish the token")
	}
	if limiter.allow("127.0.0.1") {
		t.Fatal("refill exceeded bucket capacity")
	}
}
