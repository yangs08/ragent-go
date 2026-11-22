package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

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
