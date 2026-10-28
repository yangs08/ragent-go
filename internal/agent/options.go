package agent

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

func LoadOptions(getenv func(string) string) (Options, error) {
	options := Options{MaxIterations: 4, MaxToolCalls: 3, Timeout: 30 * time.Second}
	for _, field := range []struct {
		key    string
		target *int
	}{
		{"RAGENT_AGENT_MAX_ITERATIONS", &options.MaxIterations},
		{"RAGENT_AGENT_MAX_TOOL_CALLS", &options.MaxToolCalls},
	} {
		if value := getenv(field.key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return Options{}, fmt.Errorf("%s must be an integer", field.key)
			}
			*field.target = parsed
		}
	}
	if value := getenv("RAGENT_AGENT_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return Options{}, errors.New("RAGENT_AGENT_TIMEOUT must be a duration")
		}
		options.Timeout = timeout
	}
	if err := options.validate(); err != nil {
		return Options{}, err
	}
	return options, nil
}

func (o Options) validate() error {
	if o.MaxIterations < 1 || o.MaxIterations > 20 || o.MaxToolCalls < 1 || o.MaxToolCalls > 20 || o.Timeout <= 0 || o.Timeout > 5*time.Minute {
		return errors.New("agent requires 1..20 iterations, 1..20 tool calls, and a timeout in (0,5m]")
	}
	return nil
}
