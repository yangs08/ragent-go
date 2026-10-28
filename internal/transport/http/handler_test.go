package httptransport

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	var logs bytes.Buffer
	handler := NewHandler(slog.New(slog.NewJSONHandler(&logs, nil)))
	req := httptest.NewRequest(http.MethodGet, "/healthz?token=do-not-log", nil)
	req.Header.Set("X-Request-ID", "untrusted-client-id")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("missing content type")
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body=%v", body)
	}
	id := response.Header().Get("X-Request-ID")
	if id == "" || id == "untrusted-client-id" {
		t.Fatalf("request id=%q", id)
	}
	if !strings.Contains(logs.String(), id) || strings.Contains(logs.String(), "do-not-log") {
		t.Fatalf("unexpected logs: %s", logs.String())
	}
}

func TestHealthRoutes(t *testing.T) {
	handler := NewHandler(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	for _, tt := range []struct {
		method, path string
		status       int
	}{{"GET", "/missing", 404}, {"POST", "/healthz", 405}, {"GET", "/healthz/extra", 404}, {"HEAD", "/healthz", 200}} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			r := httptest.NewRecorder()
			handler.ServeHTTP(r, httptest.NewRequest(tt.method, tt.path, nil))
			if r.Code != tt.status {
				t.Fatalf("status=%d", r.Code)
			}
		})
	}
}
