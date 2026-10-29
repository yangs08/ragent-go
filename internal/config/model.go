package config

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
)

type ModelConfig struct {
	BaseURL string
	Model   string
	APIKey  string
	Timeout time.Duration
}

func loadModel(getenv func(string) string) (ModelConfig, error) {
	cfg := ModelConfig{BaseURL: strings.TrimSpace(getenv("RAGENT_MODEL_BASE_URL")), Model: strings.TrimSpace(getenv("RAGENT_MODEL_NAME")), APIKey: strings.TrimSpace(getenv("RAGENT_MODEL_API_KEY")), Timeout: 20 * time.Second}
	if value := getenv("RAGENT_MODEL_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return ModelConfig{}, errors.New("RAGENT_MODEL_TIMEOUT must be a duration")
		}
		cfg.Timeout = timeout
	}
	if cfg.BaseURL == "" && cfg.Model == "" && cfg.APIKey == "" {
		if cfg.Timeout <= 0 || cfg.Timeout > 5*time.Minute {
			return ModelConfig{}, errors.New("RAGENT_MODEL_TIMEOUT must be greater than zero and at most 5m")
		}
		return cfg, nil
	}
	if err := cfg.Validate(); err != nil {
		return ModelConfig{}, err
	}
	return cfg, nil
}

func (s ModelConfig) Validate() error {
	if s.Model == "" {
		return errors.New("RAGENT_MODEL_NAME is required")
	}
	if s.APIKey == "" {
		return errors.New("RAGENT_MODEL_API_KEY is required")
	}
	if s.Timeout <= 0 || s.Timeout > 5*time.Minute {
		return errors.New("RAGENT_MODEL_TIMEOUT must be greater than zero and at most 5m")
	}
	endpoint, err := url.Parse(s.BaseURL)
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("RAGENT_MODEL_BASE_URL must be a base URL without credentials, query or fragment")
	}
	if endpoint.Scheme != "https" {
		ip := net.ParseIP(endpoint.Hostname())
		loopback := endpoint.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
		if endpoint.Scheme != "http" || !loopback {
			return errors.New("RAGENT_MODEL_BASE_URL requires HTTPS except on loopback")
		}
	}
	return nil
}
