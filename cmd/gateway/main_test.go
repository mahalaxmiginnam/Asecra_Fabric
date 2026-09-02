package main

import (
	"context"

	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/plugin"
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

func TestServerShutdownCompletes(t *testing.T) {
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestGatewayGeneratesRequestID(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	)
	defer upstream.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Address:        ":0",
			RequestTimeout: time.Second,
		},
		Upstreams: map[string]string{
			"default": upstream.URL,
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
		t.Fatalf("newHandlerWithConfig() error = %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
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

	requestID := recorder.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("expected X-Request-ID response header")
	}
}

func TestGatewayPreservesRequestID(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Address:        ":0",
			RequestTimeout: time.Second,
		},
		Upstreams: map[string]string{
			"default": upstream.URL,
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
		t.Fatalf("newHandlerWithConfig() error = %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
		nil,
	)
	req.Header.Set("X-Request-ID", "test-request-123")

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if got := recorder.Header().Get("X-Request-ID"); got != "test-request-123" {
		t.Fatalf(
			"X-Request-ID = %q, want %q",
			got,
			"test-request-123",
		)
	}
}

func TestGatewayForwardsRequestIDToUpstream(t *testing.T) {
	const requestID = "test-request-456"

	var receivedRequestID string

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedRequestID = r.Header.Get("X-Request-ID")
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Address:        ":0",
			RequestTimeout: time.Second,
		},
		Upstreams: map[string]string{
			"default": upstream.URL,
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
		t.Fatalf("newHandlerWithConfig() error = %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
		nil,
	)
	req.Header.Set("X-Request-ID", requestID)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if receivedRequestID != requestID {
		t.Fatalf(
			"upstream X-Request-ID = %q, want %q",
			receivedRequestID,
			requestID,
		)
	}
}

func TestGatewayRejectsRequestFromPolicy(t *testing.T) {
	upstreamCalled := false

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upstreamCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Address:        ":0",
			RequestTimeout: time.Second,
		},
		Upstreams: map[string]string{
			"default": upstream.URL,
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

	pipeline := plugin.NewPipeline(
		plugin.PolicyComponent(
			plugin.MethodPolicy{
				AllowedMethods: map[string]bool{
					http.MethodPost: true,
				},
			},
		),
	)

	handler, err := newHandlerWithConfigAndPipeline(
		cfg,
		pipeline,
	)
	if err != nil {
		t.Fatalf(
			"newHandlerWithConfigAndPipeline() error = %v",
			err,
		)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if got := recorder.Body.String(); got != "method not allowed\n" {
		t.Fatalf(
			"body = %q, want %q",
			got,
			"method not allowed\n",
		)
	}

	if upstreamCalled {
		t.Fatal("upstream was called after policy rejection")
	}
}

func TestGatewayExecutesPluginBeforeUpstream(t *testing.T) {
	var pluginRoute string
	var pluginMethod string

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("upstream-ok"))
		}),
	)
	defer upstream.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Address:        ":0",
			RequestTimeout: time.Second,
		},
		Upstreams: map[string]string{
			"default": upstream.URL,
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

	pipeline := plugin.NewPipeline(
		plugin.PluginComponent(
			testGatewayRecordingPlugin{
				onExecute: func(ctx *plugin.Context) {
					pluginRoute = ctx.Route.Name
					pluginMethod = ctx.Request.Method
				},
			},
		),
	)

	handler, err := newHandlerWithConfigAndPipeline(
		cfg,
		pipeline,
	)
	if err != nil {
		t.Fatalf(
			"newHandlerWithConfigAndPipeline() error = %v",
			err,
		)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
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

	if got := recorder.Body.String(); got != "upstream-ok" {
		t.Fatalf(
			"body = %q, want %q",
			got,
			"upstream-ok",
		)
	}

	if pluginRoute != "api" {
		t.Fatalf(
			"plugin route = %q, want %q",
			pluginRoute,
			"api",
		)
	}

	if pluginMethod != http.MethodGet {
		t.Fatalf(
			"plugin method = %q, want %q",
			pluginMethod,
			http.MethodGet,
		)
	}
}

type testGatewayRecordingPlugin struct {
	onExecute func(*plugin.Context)
}

func (p testGatewayRecordingPlugin) Name() string {
	return "gateway-recording-plugin"
}

func (p testGatewayRecordingPlugin) Execute(
	ctx *plugin.Context,
) (plugin.Result, error) {
	p.onExecute(ctx)

	return plugin.Result{
		Outcome: plugin.Continue,
	}, nil
}
