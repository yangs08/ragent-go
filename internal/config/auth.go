package config

import (
	"errors"
	"strconv"
	"time"
)

type AuthConfig struct {
	SessionTTL, RequestTimeout, LoginWindow time.Duration
	LoginAttempts                           int
}

func loadAuth(getenv func(string) string) (AuthConfig, error) {
	cfg := AuthConfig{SessionTTL: 24 * time.Hour, RequestTimeout: 5 * time.Second, LoginWindow: time.Minute, LoginAttempts: 10}
	for key, target := range map[string]*time.Duration{"RAGENT_AUTH_SESSION_TTL": &cfg.SessionTTL, "RAGENT_AUTH_REQUEST_TIMEOUT": &cfg.RequestTimeout, "RAGENT_AUTH_LOGIN_WINDOW": &cfg.LoginWindow} {
		if v := getenv(key); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return AuthConfig{}, errors.New(key + " must be a duration")
			}
			*target = d
		}
	}
	if v := getenv("RAGENT_AUTH_LOGIN_ATTEMPTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return AuthConfig{}, errors.New("RAGENT_AUTH_LOGIN_ATTEMPTS must be an integer")
		}
		cfg.LoginAttempts = n
	}
	if err := cfg.Validate(); err != nil {
		return AuthConfig{}, err
	}
	return cfg, nil
}
func (c AuthConfig) Validate() error {
	if c.SessionTTL < time.Minute || c.SessionTTL > 7*24*time.Hour || c.RequestTimeout <= 0 || c.RequestTimeout > time.Minute || c.LoginWindow < time.Second || c.LoginWindow > time.Hour || c.LoginAttempts < 1 || c.LoginAttempts > 100 {
		return errors.New("invalid auth configuration: session 1m..7d, request (0,1m], login window 1s..1h, attempts 1..100")
	}
	return nil
}
