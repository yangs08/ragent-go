package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/define"
)

func TestRecoveryResponse(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "uncommitted"
		if committed {
			name = "committed"
		}
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			r.Use(Recovery())
			r.GET("/", func(c *gin.Context) {
				c.Header("X-Request-ID", "panic-test")
				if committed {
					c.Status(202)
					_, _ = c.Writer.WriteString("partial")
				}
				panic("private-detail")
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			if committed {
				if w.Code != 202 || w.Body.String() != "partial" {
					t.Fatalf("overwrote committed response: %d %s", w.Code, w.Body)
				}
				return
			}
			var result define.Result
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Code != 500 || result.Code != define.CodeServerError || result.Message != "internal_server_error" || result.RequestID != "panic-test" || result.Data != nil {
				t.Fatalf("unexpected panic response: %d %+v", w.Code, result)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing cache policy")
			}
		})
	}
}
