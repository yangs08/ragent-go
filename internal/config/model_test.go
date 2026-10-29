package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadModel(t *testing.T) {
	base := map[string]string{"RAGENT_MODEL_BASE_URL": "https://model.example/v1", "RAGENT_MODEL_NAME": "test-model", "RAGENT_MODEL_API_KEY": "private-value"}
	tests := []struct {
		name, key, value string
		invalid          bool
	}{
		{name: "valid"},
		{name: "custom timeout", key: "RAGENT_MODEL_TIMEOUT", value: "2s"},
		{name: "missing url", key: "RAGENT_MODEL_BASE_URL", invalid: true},
		{name: "missing model", key: "RAGENT_MODEL_NAME", invalid: true},
		{name: "missing key", key: "RAGENT_MODEL_API_KEY", invalid: true},
		{name: "url parse error", key: "RAGENT_MODEL_BASE_URL", value: "%", invalid: true},
		{name: "url fragment", key: "RAGENT_MODEL_BASE_URL", value: "https://model.example/v1#private-value", invalid: true},
		{name: "invalid url", key: "RAGENT_MODEL_BASE_URL", value: "not-a-url", invalid: true},
		{name: "unsafe http", key: "RAGENT_MODEL_BASE_URL", value: "http://model.example/v1", invalid: true},
		{name: "url credentials", key: "RAGENT_MODEL_BASE_URL", value: "https://private-value@model.example/v1", invalid: true},
		{name: "url query", key: "RAGENT_MODEL_BASE_URL", value: "https://model.example/v1?key=private-value", invalid: true},
		{name: "local http", key: "RAGENT_MODEL_BASE_URL", value: "http://127.0.0.1:9000/v1"},
		{name: "invalid timeout", key: "RAGENT_MODEL_TIMEOUT", value: "bad", invalid: true},
		{name: "zero timeout", key: "RAGENT_MODEL_TIMEOUT", value: "0s", invalid: true},
		{name: "long timeout", key: "RAGENT_MODEL_TIMEOUT", value: "6m", invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range base {
				env[k] = v
			}
			if tt.key != "" {
				env[tt.key] = tt.value
			}
			cfg, err := loadModel(func(k string) string { return env[k] })
			if tt.invalid {
				if err == nil {
					t.Fatal("expected error")
				}
				if strings.Contains(err.Error(), "private-value") {
					t.Fatal("secret leaked")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "valid" && cfg.Timeout != 20*time.Second {
				t.Fatalf("timeout=%v", cfg.Timeout)
			}
			if tt.name == "custom timeout" && cfg.Timeout != 2*time.Second {
				t.Fatalf("timeout=%v", cfg.Timeout)
			}
		})
	}
}
