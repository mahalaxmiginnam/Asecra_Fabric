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
	gatewayAddress, err := getStringEnv(
		"ASECRA_GATEWAY_ADDR",
		":8080",
	)
	if err != nil {
		return Config{}, err
	}

	requestTimeout, err := getDurationEnv(
		"ASECRA_REQUEST_TIMEOUT",
		2*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	upstreamURL := getEnv(
		"ASECRA_UPSTREAM_URL",
		"http://localhost:9000",
	)

	retryMaxAttempts, err := getIntEnv(
		"ASECRA_RETRY_MAX_ATTEMPTS",
		3,
	)
	if err != nil {
		return Config{}, err
	}

	retryBaseDelay, err := getDurationEnv(
		"ASECRA_RETRY_BASE_DELAY",
		100*time.Millisecond,
	)
	if err != nil {
		return Config{}, err
	}

	circuitFailureThreshold, err := getIntEnv(
		"ASECRA_CIRCUIT_FAILURE_THRESHOLD",
		3,
	)
	if err != nil {
		return Config{}, err
	}

	circuitResetTimeout, err := getDurationEnv(
		"ASECRA_CIRCUIT_RESET_TIMEOUT",
		10*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	idempotencyTTL, err := getDurationEnv(
		"ASECRA_IDEMPOTENCY_TTL",
		10*time.Minute,
	)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Gateway: GatewayConfig{
			Address:        gatewayAddress,
			RequestTimeout: requestTimeout,
		},

		Upstreams: map[string]string{
			"default": upstreamURL,
		},

		Retry: RetryConfig{
			MaxAttempts: retryMaxAttempts,
			BaseDelay:   retryBaseDelay,
		},

		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: circuitFailureThreshold,
			ResetTimeout:     circuitResetTimeout,
		},

		Idempotency: IdempotencyConfig{
			TTL: idempotencyTTL,
		},
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Gateway.Address == "" {
		return fmt.Errorf(
			"gateway address cannot be empty",
		)
	}

	if cfg.Gateway.RequestTimeout <= 0 {
		return fmt.Errorf(
			"request timeout must be greater than zero",
		)
	}

	if len(cfg.Upstreams) == 0 {
		return fmt.Errorf(
			"at least one upstream must be configured",
		)
	}

	for name, upstreamURL := range cfg.Upstreams {
		if name == "" {
			return fmt.Errorf(
				"upstream name cannot be empty",
			)
		}

		parsedURL, err := url.ParseRequestURI(upstreamURL)
		if err != nil {
			return fmt.Errorf(
				"invalid upstream URL for %q: %w",
				name,
				err,
			)
		}

		if parsedURL.Scheme != "http" &&
			parsedURL.Scheme != "https" {
			return fmt.Errorf(
				"upstream %q must use http or https",
				name,
			)
		}

		if parsedURL.Host == "" {
			return fmt.Errorf(
				"upstream %q must include a host",
				name,
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

func getStringEnv(
	key string,
	defaultValue string,
) (string, error) {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue, nil
	}

	return value, nil
}

func getIntEnv(
	key string,
	defaultValue int,
) (int, error) {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid integer for %s: %w",
			key,
			err,
		)
	}

	return parsed, nil
}

func getDurationEnv(
	key string,
	defaultValue time.Duration,
) (time.Duration, error) {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid duration for %s: %w",
			key,
			err,
		)
	}

	return parsed, nil
}
