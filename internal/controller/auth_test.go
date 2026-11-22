package controller

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ragent-go/internal/define"
)

func TestLoginRejectsInvalidUsernameBeforeService(t *testing.T) {
	for _, username := range []string{"_alice", "用户abc", "alice smith", "alice@host"} {
		t.Run(username, func(t *testing.T) {
			body, err := json.Marshal(define.LoginRequest{Username: username, Password: "valid-password"})
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			r.POST("/auth/login", Auth.Login)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			// 不初始化数据库；若漏掉入口格式判断，调用 Service 会失败。
			r.ServeHTTP(w, req)
			var result define.Result
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || result.Code != define.CodeClientError || result.Message != "用户名或密码错误" {
				t.Fatalf("unexpected response: %d %+v", w.Code, result)
			}
		})
	}
}
