package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
)

func TestGatewayRoutesToMultipleUpstreams(t *testing.T) {
	defaultServer := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.URL.Path != "/customers" {
				t.Errorf(
					"default upstream received path %q, want %q",
					r.URL.Path,
					"/customers",
				)
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("default-upstream"))
		}),
	)
	defer defaultServer.Close()

	ordersServer := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.URL.Path != "/123" {
				t.Errorf(
					"orders upstream received path %q, want %q",
					r.URL.Path,
					"/123",
				)
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("orders-upstream"))
		}),
	)
	defer ordersServer.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Address:        ":0",
			RequestTimeout: time.Second,
		},
		Upstreams: map[string]string{
			"default": defaultServer.URL,
			"orders":  ordersServer.URL,
		},
		Retry: config.RetryConfig{
			MaxAttempts: 1,
			BaseDelay:   0,
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeout:     time.Second,
		},
		Idempotency: config.IdempotencyConfig{
			TTL: time.Minute,
		},
	}

	handler, err := newHandlerWithConfig(cfg)
	if err != nil {
		t.Fatalf(
			"newHandlerWithConfig() error = %v",
			err,
		)
	}

	t.Run("orders route", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"http://gateway.local/api/orders/123",
			nil,
		)

		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"status = %d, want %d",
				recorder.Code,
				http.StatusOK,
			)
		}

		body, err := io.ReadAll(
			recorder.Result().Body,
		)
		if err != nil {
			t.Fatalf(
				"failed to read response body: %v",
				err,
			)
		}

		if string(body) != "orders-upstream" {
			t.Fatalf(
				"body = %q, want %q",
				string(body),
				"orders-upstream",
			)
		}
	})

	t.Run("default route", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet,
			"http://gateway.local/api/customers",
			nil,
		)

		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"status = %d, want %d",
				recorder.Code,
				http.StatusOK,
			)
		}

		body, err := io.ReadAll(
			recorder.Result().Body,
		)
		if err != nil {
			t.Fatalf(
				"failed to read response body: %v",
				err,
			)
		}

		if string(body) != "default-upstream" {
			t.Fatalf(
				"body = %q, want %q",
				string(body),
				"default-upstream",
			)
		}
	})
}
