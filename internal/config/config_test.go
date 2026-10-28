package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "defaults"},
		{name: "custom", env: map[string]string{"RAGENT_HTTP_ADDR": "127.0.0.1:9090", "RAGENT_SHUTDOWN_TIMEOUT": "2s", "RAGENT_LOG_LEVEL": "debug"}},
		{name: "bad address", env: map[string]string{"RAGENT_HTTP_ADDR": "invalid"}, wantErr: "RAGENT_HTTP_ADDR"},
		{name: "empty host", env: map[string]string{"RAGENT_HTTP_ADDR": ":8080"}, wantErr: "RAGENT_HTTP_ADDR"},
		{name: "zero port", env: map[string]string{"RAGENT_HTTP_ADDR": "127.0.0.1:0"}, wantErr: "RAGENT_HTTP_ADDR"},
		{name: "large port", env: map[string]string{"RAGENT_HTTP_ADDR": "127.0.0.1:65536"}, wantErr: "RAGENT_HTTP_ADDR"},
		{name: "bad port", env: map[string]string{"RAGENT_HTTP_ADDR": "localhost:abc"}, wantErr: "RAGENT_HTTP_ADDR"},
		{name: "bad duration", env: map[string]string{"RAGENT_SHUTDOWN_TIMEOUT": "bad"}, wantErr: "RAGENT_SHUTDOWN_TIMEOUT"},
		{name: "zero duration", env: map[string]string{"RAGENT_SHUTDOWN_TIMEOUT": "0s"}, wantErr: "RAGENT_SHUTDOWN_TIMEOUT"},
		{name: "negative duration", env: map[string]string{"RAGENT_SHUTDOWN_TIMEOUT": "-1s"}, wantErr: "RAGENT_SHUTDOWN_TIMEOUT"},
		{name: "long duration", env: map[string]string{"RAGENT_SHUTDOWN_TIMEOUT": "2m"}, wantErr: "RAGENT_SHUTDOWN_TIMEOUT"},
		{name: "bad level", env: map[string]string{"RAGENT_LOG_LEVEL": "secret-value"}, wantErr: "RAGENT_LOG_LEVEL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(func(k string) string { return tt.env[k] })
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error=%v", err)
				}
				if strings.Contains(err.Error(), "secret-value") {
					t.Fatal("configuration value leaked")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "defaults" && (cfg.HTTPAddr != "127.0.0.1:8080" || cfg.ShutdownTimeout != 5*time.Second || cfg.LogLevel != slog.LevelInfo) {
				t.Fatalf("defaults=%+v", cfg)
			}
			if tt.name == "custom" && (cfg.HTTPAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 2*time.Second || cfg.LogLevel != slog.LevelDebug) {
				t.Fatalf("custom=%+v", cfg)
			}
		})
	}
}
