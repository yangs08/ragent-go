package config

import (
	"testing"
	"time"
)

func TestLoadAgent(t *testing.T) {
	for _, tt := range []struct {
		name, key, value string
		invalid          bool
	}{
		{name: "defaults"}, {name: "iterations", key: "RAGENT_AGENT_MAX_ITERATIONS", value: "6"},
		{name: "calls", key: "RAGENT_AGENT_MAX_TOOL_CALLS", value: "2"},
		{name: "timeout", key: "RAGENT_AGENT_TIMEOUT", value: "2s"},
		{name: "bad integer", key: "RAGENT_AGENT_MAX_ITERATIONS", value: "oops", invalid: true},
		{name: "zero iterations", key: "RAGENT_AGENT_MAX_ITERATIONS", value: "0", invalid: true},
		{name: "large calls", key: "RAGENT_AGENT_MAX_TOOL_CALLS", value: "21", invalid: true},
		{name: "bad calls", key: "RAGENT_AGENT_MAX_TOOL_CALLS", value: "bad", invalid: true},
		{name: "bad timeout", key: "RAGENT_AGENT_TIMEOUT", value: "bad", invalid: true},
		{name: "zero timeout", key: "RAGENT_AGENT_TIMEOUT", value: "0s", invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := loadAgent(func(k string) string {
				if k == tt.key {
					return tt.value
				}
				return ""
			})
			if tt.invalid {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "defaults" && (opts.MaxIterations != 4 || opts.MaxToolCalls != 3 || opts.Timeout != 30*time.Second) {
				t.Fatalf("defaults=%+v", opts)
			}
			if tt.name == "iterations" && opts.MaxIterations != 6 {
				t.Fatal(opts)
			}
			if tt.name == "calls" && opts.MaxToolCalls != 2 {
				t.Fatal(opts)
			}
			if tt.name == "timeout" && opts.Timeout != 2*time.Second {
				t.Fatal(opts)
			}
		})
	}
}
