package gateway

import (
	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
)

func testUpstreamConfig() config.Config {
	return config.Config{
		Gateway: config.GatewayConfig{
			RequestTimeout: 2 * time.Second,
		},
		Retry: config.RetryConfig{
			MaxAttempts: 1,
			BaseDelay:   100 * time.Millisecond,
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeout:     10 * time.Second,
		},
	}
}

func TestNewUpstreamRuntimeRejectsUnsupportedScheme(t *testing.T) {
	cfg := testUpstreamConfig()

	_, err := NewUpstreamRuntime(
		"orders",
		"ftp://orders.internal",
		cfg,
		metrics.New(),
	)
	if err == nil {
		t.Fatal("expected unsupported scheme error")
	}
}

func TestNewUpstreamRuntimeRejectsMissingHost(t *testing.T) {
	cfg := testUpstreamConfig()

	_, err := NewUpstreamRuntime(
		"orders",
		"http:///orders",
		cfg,
		metrics.New(),
	)
	if err == nil {
		t.Fatal("expected missing host error")
	}
}

func TestNewUpstreamRuntimeCreatesRuntime(t *testing.T) {
	cfg := testUpstreamConfig()

	runtime, err := NewUpstreamRuntime(
		"orders",
		"http://orders.internal/v1",
		cfg,
		metrics.New(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runtime.Name != "orders" {
		t.Fatalf(
			"expected runtime name orders, got %q",
			runtime.Name,
		)
	}

	if runtime.URL.String() != "http://orders.internal/v1" {
		t.Fatalf(
			"unexpected runtime URL %q",
			runtime.URL.String(),
		)
	}

	if runtime.Executor == nil {
		t.Fatal("expected executor")
	}

	if runtime.Executor.Breaker == nil {
		t.Fatal("expected circuit breaker")
	}
}

func TestNewUpstreamRuntimeCreatesIndependentRuntimes(t *testing.T) {
	cfg := testUpstreamConfig()
	metricsCollector := metrics.New()

	orders, err := NewUpstreamRuntime(
		"orders",
		"http://orders.internal",
		cfg,
		metricsCollector,
	)
	if err != nil {
		t.Fatalf("failed to create orders runtime: %v", err)
	}

	defaultRuntime, err := NewUpstreamRuntime(
		"default",
		"http://default.internal",
		cfg,
		metricsCollector,
	)
	if err != nil {
		t.Fatalf("failed to create default runtime: %v", err)
	}

	if orders.Executor == defaultRuntime.Executor {
		t.Fatal("expected independent executors")
	}

	if orders.Executor.Breaker == defaultRuntime.Executor.Breaker {
		t.Fatal("expected independent circuit breakers")
	}

	if orders.URL == defaultRuntime.URL {
		t.Fatal("expected independent upstream URLs")
	}
}
