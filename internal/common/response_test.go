package common

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
)

func TestFailureResponsesAbortAndRedact(t *testing.T) {
	old := bootstrap.Logger
	defer func() { bootstrap.Logger = old }()
	var logs bytes.Buffer
	bootstrap.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	cause := &pgconn.PgError{Code: "08006", Message: "private-token-and-sql"}
	failure := define.ServerError("认证服务暂不可用", "find login user", cause)
	if !errors.Is(failure, cause) {
		t.Fatal("lost cause")
	}
	r := gin.New()
	reached := false
	r.Use(func(c *gin.Context) { c.Header("X-Request-ID", "test-request"); Error(c, failure) })
	r.GET("/", func(c *gin.Context) { reached = true })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if reached || w.Code != 200 || !strings.Contains(w.Body.String(), define.CodeServerError) {
		t.Fatal("failure did not abort with business response")
	}
	if strings.Contains(w.Body.String()+logs.String(), cause.Message) {
		t.Fatal("private cause leaked")
	}
	for _, want := range []string{"08006", "find login user", "test-request"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("missing diagnostic %s", want)
		}
	}
}
func TestNoAuthAborts(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	NoAuth(c)
	if !c.IsAborted() || w.Code != 200 || !strings.Contains(w.Body.String(), define.CodeClientError) {
		t.Fatal("invalid unauthenticated response")
	}
	if !strings.Contains(w.Body.String(), "未登录或登录已过期") {
		t.Fatal("missing message")
	}
}

func TestErrorPreservesCommittedResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Status(202)
	_, _ = c.Writer.WriteString("partial")
	Error(c, define.ClientError("rejected"))
	if !c.IsAborted() || w.Code != 202 || w.Body.String() != "partial" {
		t.Fatal("error replaced or appended to committed output")
	}
}
