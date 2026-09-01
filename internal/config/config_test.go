package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ASECRA_GATEWAY_ADDR", "")
	t.Setenv("ASECRA_UPSTREAM_URL", "")
	t.Setenv("ASECRA_REQUEST_TIMEOUT", "")
	t.Setenv("ASECRA_RETRY_MAX_ATTEMPTS", "")
	t.Setenv("ASECRA_RETRY_BASE_DELAY", "")
	t.Setenv("ASECRA_CIRCUIT_FAILURE_THRESHOLD", "")
	t.Setenv("ASECRA_CIRCUIT_RESET_TIMEOUT", "")
	t.Setenv("ASECRA_IDEMPOTENCY_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != ":8080" {
		t.Fatalf(
			"expected :8080, got %s",
			cfg.Gateway.Address,
		)
	}

	if cfg.Upstream.URL != "http://localhost:9000" {
		t.Fatalf(
			"unexpected upstream URL: %s",
			cfg.Upstream.URL,
		)
	}

	if cfg.Gateway.RequestTimeout != 2*time.Second {
		t.Fatalf(
			"unexpected request timeout: %v",
			cfg.Gateway.RequestTimeout,
		)
	}

	if cfg.Retry.MaxAttempts != 3 {
		t.Fatalf(
			"unexpected max attempts: %d",
			cfg.Retry.MaxAttempts,
		)
	}
}

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv(
		"ASECRA_GATEWAY_ADDR",
		":9090",
	)

	t.Setenv(
		"ASECRA_UPSTREAM_URL",
		"http://localhost:9999",
	)

	t.Setenv(
		"ASECRA_REQUEST_TIMEOUT",
		"5s",
	)

	t.Setenv(
		"ASECRA_RETRY_MAX_ATTEMPTS",
		"5",
	)

	t.Setenv(
		"ASECRA_RETRY_BASE_DELAY",
		"250ms",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gateway.Address != ":9090" {
		t.Fatalf(
			"expected :9090, got %s",
			cfg.Gateway.Address,
		)
	}

	if cfg.Upstream.URL != "http://localhost:9999" {
		t.Fatalf(
			"unexpected upstream URL: %s",
			cfg.Upstream.URL,
		)
	}

	if cfg.Gateway.RequestTimeout != 5*time.Second {
		t.Fatalf(
			"unexpected timeout: %v",
			cfg.Gateway.RequestTimeout,
		)
	}

	if cfg.Retry.MaxAttempts != 5 {
		t.Fatalf(
			"unexpected max attempts: %d",
			cfg.Retry.MaxAttempts,
		)
	}

	if cfg.Retry.BaseDelay != 250*time.Millisecond {
		t.Fatalf(
			"unexpected base delay: %v",
			cfg.Retry.BaseDelay,
		)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv(
		"ASECRA_RETRY_MAX_ATTEMPTS",
		"0",
	)

	_, err := Load()

	if err == nil {
		t.Fatal("expected configuration validation error")
	}
}
