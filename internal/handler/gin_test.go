package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/config"
)

func ginRouter(t *testing.T, output io.Writer) *gin.Engine {
	t.Helper()
	router, ok := NewHandler(slog.New(slog.NewJSONHandler(output, nil)), nil, config.AuthConfig{}).(*gin.Engine)
	if !ok {
		t.Fatal("HTTP handler must be backed by Gin")
	}
	return router
}

func TestGinHealthProtocol(t *testing.T) {
	for _, tt := range []struct {
		method, path string
		want         int
	}{{"GET", "/healthz", 200}, {"HEAD", "/healthz", 200}, {"POST", "/healthz", 405}, {"GET", "/healthz/", 404}} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			var logs bytes.Buffer
			handler := NewHandler(slog.New(slog.NewJSONHandler(&logs, nil)), nil, config.AuthConfig{})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(tt.method, tt.path, nil))
			if response.Code != tt.want {
				t.Fatalf("status=%d want %d", response.Code, tt.want)
			}
			if tt.method == "HEAD" && response.Body.Len() != 0 {
				t.Fatal("HEAD must not contain a response body")
			}
			if tt.method == "POST" {
				allow := response.Header().Get("Allow")
				if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
					t.Fatalf("Allow=%q", allow)
				}
			}
			var event struct {
				Status    int    `json:"status"`
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
				t.Fatal(err)
			}
			if event.Status != tt.want || event.RequestID != response.Header().Get("X-Request-ID") {
				t.Fatalf("event=%+v", event)
			}
		})
	}
}

func TestGinRecoveryDoesNotExposeRequestOrPanic(t *testing.T) {
	var logs bytes.Buffer
	router := ginRouter(t, &logs)
	router.GET("/test-panic", func(c *gin.Context) { panic("private-panic-value") })
	request := httptest.NewRequest("GET", "/test-panic?token=private-query-value", nil)
	request.Header.Set("Authorization", "Bearer private-header-value")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 500 {
		t.Fatalf("status=%d", response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request id")
	}
	if !strings.Contains(response.Body.String(), "internal_server_error") {
		t.Fatalf("body=%s", response.Body.String())
	}
	for _, secret := range []string{"private-panic-value", "private-query-value", "private-header-value"} {
		if strings.Contains(logs.String()+response.Body.String(), secret) {
			t.Fatal("sensitive content leaked")
		}
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Fatalf("missing recovered status: %s", logs.String())
	}
}

func TestGinRecoveryPreservesCommittedResponse(t *testing.T) {
	router := ginRouter(t, io.Discard)
	router.GET("/test-partial", func(c *gin.Context) {
		c.Status(http.StatusAccepted)
		_, _ = io.WriteString(c.Writer, "partial")
		c.Writer.Flush()
		panic("private-panic-value")
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/test-partial", nil))
	if response.Code != 202 || response.Body.String() != "partial" || !response.Flushed {
		t.Fatalf("response=%d %q flushed=%v", response.Code, response.Body.String(), response.Flushed)
	}
}

func TestGinDoesNotTrustForwardedClientIP(t *testing.T) {
	router := ginRouter(t, io.Discard)
	router.GET("/test-ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })
	request := httptest.NewRequest("GET", "/test-ip", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Body.String() != "192.0.2.10" {
		t.Fatalf("IP=%s", response.Body.String())
	}
}
