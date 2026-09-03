package gateway

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/plugin"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/router"
)

func testConfig(upstreams map[string]string) config.Config {
	return config.Config{
		Gateway: config.GatewayConfig{
			RequestTimeout: time.Second,
		},
		Upstreams: upstreams,
		Retry: config.RetryConfig{
			MaxAttempts: 1,
			BaseDelay:   0,
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeout:     time.Second,
		},
	}
}

func newTestGateway(
	t *testing.T,
	cfg config.Config,
	pipeline *plugin.Pipeline,
) *Gateway {
	t.Helper()

	requestRouter := router.NewRouter([]router.Route{
		{
			Name:     "orders",
			Prefix:   "/api/orders",
			Upstream: "orders",
		},
		{
			Name:     "api",
			Prefix:   "/api",
			Upstream: "default",
		},
	})

	metricsCollector := metrics.New()

	gatewayRuntime, err := New(
		cfg,
		requestRouter,
		pipeline,
		metricsCollector,
	)
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}

	return gatewayRuntime
}

func TestGatewayRoutesOrdersToOrdersUpstream(t *testing.T) {
	var receivedPath string

	ordersBackend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("orders"))
		}),
	)
	defer ordersBackend.Close()

	defaultBackend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("default upstream should not receive orders request")
		}),
	)
	defer defaultBackend.Close()

	cfg := testConfig(map[string]string{
		"orders":  ordersBackend.URL,
		"default": defaultBackend.URL,
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/orders/123",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if recorder.Body.String() != "orders" {
		t.Fatalf(
			"expected body %q, got %q",
			"orders",
			recorder.Body.String(),
		)
	}

	if receivedPath != "/123" {
		t.Fatalf(
			"expected upstream path %q, got %q",
			"/123",
			receivedPath,
		)
	}
}

func TestGatewayRoutesGeneralAPIToDefaultUpstream(t *testing.T) {
	var receivedPath string

	defaultBackend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("default"))
		}),
	)
	defer defaultBackend.Close()

	cfg := testConfig(map[string]string{
		"default": defaultBackend.URL,
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/customers/42",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if recorder.Body.String() != "default" {
		t.Fatalf(
			"expected body %q, got %q",
			"default",
			recorder.Body.String(),
		)
	}

	if receivedPath != "/customers/42" {
		t.Fatalf(
			"expected upstream path %q, got %q",
			"/customers/42",
			receivedPath,
		)
	}
}

func TestGatewayReturnsNotFoundForUnmatchedRoute(t *testing.T) {
	cfg := testConfig(map[string]string{
		"default": "http://localhost:9000",
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/unknown",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNotFound,
			recorder.Code,
		)
	}
}

type rejectingGatewayPolicy struct{}

func (rejectingGatewayPolicy) Name() string {
	return "rejecting-policy"
}

func (rejectingGatewayPolicy) Evaluate(
	*plugin.Context,
) (plugin.Result, error) {
	return plugin.Result{
		Outcome:    plugin.Reject,
		StatusCode: http.StatusForbidden,
		Message:    "request rejected",
	}, nil
}

func TestGatewayReturnsPluginRejection(t *testing.T) {
	cfg := testConfig(map[string]string{
		"default": "http://localhost:9000",
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(
			plugin.PolicyComponent(
				rejectingGatewayPolicy{},
			),
		),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/customers",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			recorder.Code,
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		"request rejected",
	) {
		t.Fatalf(
			"expected rejection message, got %q",
			recorder.Body.String(),
		)
	}
}

func TestGatewayReturnsBadGatewayWhenUpstreamFails(t *testing.T) {
	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(
				w,
				"upstream failure",
				http.StatusBadGateway,
			)
		}),
	)
	backendURL := backend.URL
	backend.Close()

	cfg := testConfig(map[string]string{
		"default": backendURL,
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/customers",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadGateway,
			recorder.Code,
		)
	}
}

func TestGatewayPreservesQueryString(t *testing.T) {
	var receivedQuery string

	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer backend.Close()

	cfg := testConfig(map[string]string{
		"default": backend.URL,
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/customers?limit=10&active=true",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if receivedQuery != "limit=10&active=true" {
		t.Fatalf(
			"expected query %q, got %q",
			"limit=10&active=true",
			receivedQuery,
		)
	}
}

func TestGatewayAcceptsUpstreamWithBasePath(t *testing.T) {
	var receivedPath string

	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL + "/v1")
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(map[string]string{
		"default": backendURL.String(),
	})

	gateway := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/customers",
		nil,
	)

	recorder := httptest.NewRecorder()

	gateway.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if receivedPath != "/v1/customers" {
		t.Fatalf(
			"expected upstream path %q, got %q",
			"/v1/customers",
			receivedPath,
		)
	}
}

func TestGatewayConcurrentUpstreamIsolation(t *testing.T) {
	ordersRequests := make(chan string, 100)
	defaultRequests := make(chan string, 100)

	ordersServer := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		ordersRequests <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ordersServer.Close()

	defaultServer := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		defaultRequests <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer defaultServer.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			RequestTimeout: 2 * time.Second,
		},
		Upstreams: map[string]string{
			"orders":  ordersServer.URL,
			"default": defaultServer.URL,
		},
		Retry: config.RetryConfig{
			MaxAttempts: 1,
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeout:     10 * time.Second,
		},
	}

	gatewayRuntime := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	const requestsPerRoute = 50

	var wg sync.WaitGroup
	wg.Add(requestsPerRoute * 2)

	for i := 0; i < requestsPerRoute; i++ {
		go func() {
			defer wg.Done()

			request := httptest.NewRequest(
				http.MethodGet,
				"http://gateway.test/api/orders/123",
				nil,
			)
			response := httptest.NewRecorder()

			gatewayRuntime.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Errorf(
					"orders request returned status %d",
					response.Code,
				)
			}
		}()

		go func() {
			defer wg.Done()

			request := httptest.NewRequest(
				http.MethodGet,
				"http://gateway.test/api/customers/42",
				nil,
			)
			response := httptest.NewRecorder()

			gatewayRuntime.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Errorf(
					"default request returned status %d",
					response.Code,
				)
			}
		}()
	}

	wg.Wait()

	close(ordersRequests)
	close(defaultRequests)

	for path := range ordersRequests {
		if path != "/123" {
			t.Errorf(
				"orders upstream received unexpected path %q",
				path,
			)
		}
	}

	for path := range defaultRequests {
		if path != "/customers/42" {
			t.Errorf(
				"default upstream received unexpected path %q",
				path,
			)
		}
	}
}
func TestGatewayRestoresRequestPathAfterExecution(t *testing.T) {
	var receivedPath string

	backend := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer backend.Close()

	cfg := config.Config{
		Gateway: config.GatewayConfig{
			RequestTimeout: 2 * time.Second,
		},
		Upstreams: map[string]string{
			"default": backend.URL,
		},
		Retry: config.RetryConfig{
			MaxAttempts: 1,
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeout:     10 * time.Second,
		},
	}

	gatewayRuntime := newTestGateway(
		t,
		cfg,
		plugin.NewPipeline(),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.test/api/customers/42",
		nil,
	)

	originalPath := request.URL.Path

	response := httptest.NewRecorder()

	gatewayRuntime.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if receivedPath != "/customers/42" {
		t.Fatalf(
			"upstream received path %q, want %q",
			receivedPath,
			"/customers/42",
		)
	}

	if request.URL.Path != originalPath {
		t.Fatalf(
			"request path = %q, want original path %q",
			request.URL.Path,
			originalPath,
		)
	}
}
