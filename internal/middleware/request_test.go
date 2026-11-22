package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/bootstrap"
)

func TestRequestTimeout(t *testing.T) {
	old := bootstrap.Config.Server.RequestTimeout
	t.Cleanup(func() { bootstrap.Config.Server.RequestTimeout = old })
	for _, tt := range []struct {
		name           string
		timeout        time.Duration
		parentDeadline bool
		cancelParent   bool
	}{
		{"request deadline", 10 * time.Millisecond, false, false},
		{"shorter parent deadline", time.Hour, true, false},
		{"parent cancellation", time.Hour, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bootstrap.Config.Server.RequestTimeout = tt.timeout
			type contextKey struct{}
			parent := context.WithValue(context.Background(), contextKey{}, "identity")
			cancel := func() {}
			if tt.parentDeadline {
				parent, cancel = context.WithTimeout(parent, 10*time.Millisecond)
			} else {
				parent, cancel = context.WithCancel(parent)
			}
			defer cancel()
			r := gin.New()
			r.Use(RequestTimeout())
			var seen context.Context
			r.Use(func(c *gin.Context) {
				seen = c.Request.Context()
				c.Next()
			})
			r.GET("/", func(c *gin.Context) {
				ctx := c.Request.Context()
				if ctx != seen || ctx.Value(contextKey{}) != "identity" {
					t.Fatal("request context was replaced or lost parent values")
				}
				if deadline, ok := parent.Deadline(); ok {
					got, _ := ctx.Deadline()
					if !got.Equal(deadline) {
						t.Fatal("extended parent deadline")
					}
				}
				if tt.cancelParent {
					cancel()
				}
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
					t.Fatal("request did not cancel")
				}
				want := context.DeadlineExceeded
				if tt.cancelParent {
					want = context.Canceled
				}
				if !errors.Is(ctx.Err(), want) {
					t.Fatalf("unexpected cancellation: %v", ctx.Err())
				}
				c.Status(204)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil).WithContext(parent))
			if w.Code != 204 {
				t.Fatalf("unexpected status: %d", w.Code)
			}
		})
	}
}

func TestRequestTimeoutReleasesContextOnReturn(t *testing.T) {
	old := bootstrap.Config.Server.RequestTimeout
	t.Cleanup(func() { bootstrap.Config.Server.RequestTimeout = old })
	bootstrap.Config.Server.RequestTimeout = time.Hour
	r := gin.New()
	r.Use(RequestTimeout())
	var ctx context.Context
	r.GET("/", func(c *gin.Context) { ctx = c.Request.Context(); c.Status(204) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("request context remains active after response")
	}
}
