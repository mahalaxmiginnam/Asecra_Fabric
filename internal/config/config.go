package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Gateway        GatewayConfig
	Upstreams      map[string]string
	Retry          RetryConfig
	CircuitBreaker CircuitBreakerConfig
	Idempotency    IdempotencyConfig
}

type GatewayConfig struct {
	Address        string
	RequestTimeout time.Duration
}

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

type CircuitBreakerConfig struct {
	FailureThreshold int
	ResetTimeout     time.Duration
}

type IdempotencyConfig struct {
	TTL time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Gateway: GatewayConfig{
			Address: getEnv(
				"ASECRA_GATEWAY_ADDR",
				":8080",
			),
			RequestTimeout: getDurationEnv(
				"ASECRA_REQUEST_TIMEOUT",
				2*time.Second,
			),
		},

		Upstreams: map[string]string{
			"default": getEnv(
				"ASECRA_UPSTREAM_URL",
				"http://localhost:9000",
			),
		},

		Retry: RetryConfig{
			MaxAttempts: getIntEnv(
				"ASECRA_RETRY_MAX_ATTEMPTS",
				3,
			),
			BaseDelay: getDurationEnv(
				"ASECRA_RETRY_BASE_DELAY",
				100*time.Millisecond,
			),
		},

		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: getIntEnv(
				"ASECRA_CIRCUIT_FAILURE_THRESHOLD",
				3,
			),
			ResetTimeout: getDurationEnv(
				"ASECRA_CIRCUIT_RESET_TIMEOUT",
				10*time.Second,
			),
		},

		Idempotency: IdempotencyConfig{
			TTL: getDurationEnv(
				"ASECRA_IDEMPOTENCY_TTL",
				10*time.Minute,
			),
		},
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Gateway.Address == "" {
		return fmt.Errorf("gateway address cannot be empty")
	}

	if cfg.Gateway.RequestTimeout <= 0 {
		return fmt.Errorf(
			"request timeout must be greater than zero",
		)
	}

	for name, upstreamURL := range cfg.Upstreams {
		if _, err := url.ParseRequestURI(upstreamURL); err != nil {
			return fmt.Errorf(
				"invalid upstream URL for %q: %w",
				name,
				err,
			)
		}
	}

	if cfg.Retry.MaxAttempts < 1 {
		return fmt.Errorf(
			"retry max attempts must be at least 1",
		)
	}

	if cfg.Retry.BaseDelay < 0 {
		return fmt.Errorf(
			"retry base delay cannot be negative",
		)
	}

	if cfg.CircuitBreaker.FailureThreshold < 1 {
		return fmt.Errorf(
			"circuit failure threshold must be at least 1",
		)
	}

	if cfg.CircuitBreaker.ResetTimeout <= 0 {
		return fmt.Errorf(
			"circuit reset timeout must be greater than zero",
		)
	}

	if cfg.Idempotency.TTL <= 0 {
		return fmt.Errorf(
			"idempotency TTL must be greater than zero",
		)
	}

	return nil
}

func getEnv(
	key string,
	defaultValue string,
) string {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	return value
}

func getIntEnv(
	key string,
	defaultValue int,
) int {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

func getDurationEnv(
	key string,
	defaultValue time.Duration,
) time.Duration {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}
