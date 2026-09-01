package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	GatewayAddr string
	BackendURL  string

	RequestTimeout time.Duration

	MaxAttempts int
	BaseDelay   time.Duration

	CircuitFailureThreshold int
	CircuitResetTimeout     time.Duration

	IdempotencyTTL time.Duration
}

func Load() Config {
	return Config{
		GatewayAddr: getString(
			"ASECRA_GATEWAY_ADDR",
			":8080",
		),

		BackendURL: getString(
			"ASECRA_BACKEND_URL",
			"http://localhost:9000",
		),

		RequestTimeout: getDuration(
			"ASECRA_REQUEST_TIMEOUT",
			2*time.Second,
		),

		MaxAttempts: getInt(
			"ASECRA_MAX_ATTEMPTS",
			3,
		),

		BaseDelay: getDuration(
			"ASECRA_BASE_DELAY",
			100*time.Millisecond,
		),

		CircuitFailureThreshold: getInt(
			"ASECRA_CIRCUIT_FAILURE_THRESHOLD",
			3,
		),

		CircuitResetTimeout: getDuration(
			"ASECRA_CIRCUIT_RESET_TIMEOUT",
			10*time.Second,
		),

		IdempotencyTTL: getDuration(
			"ASECRA_IDEMPOTENCY_TTL",
			10*time.Minute,
		),
	}
}

func getString(
	key string,
	defaultValue string,
) string {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	return value
}

func getInt(
	key string,
	defaultValue int,
) int {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return result
}

func getDuration(
	key string,
	defaultValue time.Duration,
) time.Duration {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return result
}