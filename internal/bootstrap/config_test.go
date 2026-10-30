package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestYAMLAndEnvironmentPriority(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "server.yaml")
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(yamlPath, []byte("http_addr: 127.0.0.1:9090\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("timeout: 2s\nmax_tool_calls: 4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("RAGENT_AGENT_MAX_TOOL_CALLS=2\nRAGENT_HTTP_ADDR=127.0.0.1:9091\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lookup, err := LookupEnvironment(envPath, func(k string) (string, bool) {
		if k == "RAGENT_HTTP_ADDR" {
			return "127.0.0.1:9092", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigDir(dir, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.HTTPAddr != "127.0.0.1:9092" || cfg.Agent.Timeout != 2*time.Second || cfg.Agent.MaxToolCalls != 2 || cfg.Database.Enabled {
		t.Fatal("configuration layer precedence failed")
	}
}

func TestYAMLRejectsSecretsAndUnsafeErrors(t *testing.T) {
	for _, item := range []struct{ name, data string }{
		{"model.yaml", "api_key: private-value\n"}, {"database.yaml", "password: private-value\n"},
		{"server.yaml", "typo: private-value\n"}, {"agent.yaml", "timeout: [private-value]\n"},
		{"server.yaml", "{}\n---\n{}\n"}, {"server.yaml", "agent: {}\n"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, item.name), []byte(item.data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := loadConfigDir(dir, func(string) (string, bool) { return "", false })
		if err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatal("invalid module YAML was accepted or leaked contents")
		}
	}
}

func TestEmptyEnvironmentMasksYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.yaml")
	if err := os.WriteFile(path, []byte("base_url: https://model.example/v1\nname: test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("RAGENT_MODEL_API_KEY=private-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lookup, err := LookupEnvironment(envPath, func(k string) (string, bool) { return "", k == "RAGENT_MODEL_API_KEY" })
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfigDir(dir, lookup)
	if err != nil || cfg.Model.APIKey != "" {
		t.Fatal("explicit empty system credential did not mask .env")
	}

}
