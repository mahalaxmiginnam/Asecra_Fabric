package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/idempotency"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/proxy"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/requestid"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

func newHandlerForBackend(backendURL string) (http.Handler, error) {
	upstream, err := configBackendURL(backendURL)
	if err != nil {
		return nil, err
	}

	breaker := circuitbreaker.New(
		3,
		10*time.Second,
	)

	executor := &proxy.Executor{
		Client: &http.Client{
			Timeout: 0,
		},

		Upstream: upstream,

		Retry: retry.Controller{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond,
		},

		Breaker: breaker,

		RequestTimeout: 2 * time.Second,
	}

	store := idempotency.NewStore(
		10 * time.Minute,
	)

	controller := idempotency.NewController(store)

	idempotencyMiddleware := idempotency.NewMiddleware(
		controller,
	)

	var handler http.Handler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)

				_, _ = w.Write([]byte("ok"))

				return
			}

			if !strings.HasPrefix(
				r.URL.Path,
				"/api/",
			) {
				http.NotFound(w, r)

				return
			}

			originalPath := r.URL.Path

			r.URL.Path = strings.TrimPrefix(
				r.URL.Path,
				"/api",
			)

			if r.URL.Path == "" {
				r.URL.Path = "/"
			}

			result, err := executor.Execute(
				r.Context(),
				r,
			)

			r.URL.Path = originalPath

			if err != nil {

				if err == circuitbreaker.ErrOpen ||
					err == circuitbreaker.ErrBusy {

					http.Error(
						w,
						"upstream temporarily unavailable",
						http.StatusServiceUnavailable,
					)

					return
				}

				http.Error(
					w,
					"upstream request failed",
					http.StatusBadGateway,
				)

				return
			}

			_ = result.WriteTo(w)
		},
	)

	handler = idempotencyMiddleware.Handler(handler)

	handler = requestid.Middleware(handler)

	return handler, nil
}

func configBackendURL(backendURL string) (*url.URL, error) {
	return url.Parse(backendURL)
}

func TestGatewayHealth(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatalf("newHandler failed: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			resp.StatusCode,
		)
	}

	if resp.Header.Get(requestid.Header) == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

func TestGatewayNotFound(t *testing.T) {
	handler, err := newHandler()
	if err != nil {
		t.Fatalf("newHandler failed: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/unknown")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf(
			"expected 404, got %d",
			resp.StatusCode,
		)
	}
}

func TestGatewayOrdersIdempotency(t *testing.T) {
	var backendCalls atomic.Int32

	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backendCalls.Add(1)

			if r.Method != http.MethodPost {
				t.Fatalf(
					"expected POST, got %s",
					r.Method,
				)
			}

			if r.URL.Path != "/orders" {
				t.Fatalf(
					"expected /orders, got %s",
					r.URL.Path,
				)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed reading body: %v", err)
			}

			if string(body) != `{"customer":"varun"}` {
				t.Fatalf(
					"unexpected body: %s",
					body,
				)
			}

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			w.WriteHeader(http.StatusCreated)

			_, _ = w.Write(
				[]byte(`{"order_id":"order-123","created":true}`),
			)
		}),
	)

	defer backend.Close()

	handler, err := newHandlerForBackend(backend.URL)
	if err != nil {
		t.Fatalf(
			"newHandlerForBackend failed: %v",
			err,
		)
	}

	gateway := httptest.NewServer(handler)
	defer gateway.Close()

	doRequest := func() *http.Response {
		req, err := http.NewRequest(
			http.MethodPost,
			gateway.URL+"/api/orders",
			strings.NewReader(`{"customer":"varun"}`),
		)
		if err != nil {
			t.Fatalf("creating request failed: %v", err)
		}

		req.Header.Set(
			"Content-Type",
			"application/json",
		)

		req.Header.Set(
			"Idempotency-Key",
			"order-123",
		)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		return resp
	}

	first := doRequest()

	firstBody, err := io.ReadAll(first.Body)
	first.Body.Close()

	if err != nil {
		t.Fatalf(
			"reading first response failed: %v",
			err,
		)
	}

	if first.StatusCode != http.StatusCreated {
		t.Fatalf(
			"expected first response 201, got %d",
			first.StatusCode,
		)
	}

	second := doRequest()

	secondBody, err := io.ReadAll(second.Body)
	second.Body.Close()

	if err != nil {
		t.Fatalf(
			"reading second response failed: %v",
			err,
		)
	}

	if second.StatusCode != http.StatusCreated {
		t.Fatalf(
			"expected replay response 201, got %d",
			second.StatusCode,
		)
	}

	if string(firstBody) != string(secondBody) {
		t.Fatalf(
			"replay body mismatch: first=%s second=%s",
			firstBody,
			secondBody,
		)
	}

	if backendCalls.Load() != 1 {
		t.Fatalf(
			"expected exactly 1 backend call, got %d",
			backendCalls.Load(),
		)
	}
}

func TestGatewayConcurrentIdempotency(t *testing.T) {
	var backendCalls atomic.Int32

	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backendCalls.Add(1)

			time.Sleep(100 * time.Millisecond)

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			w.WriteHeader(http.StatusCreated)

			_, _ = w.Write(
				[]byte(`{"created":true}`),
			)
		}),
	)

	defer backend.Close()

	handler, err := newHandlerForBackend(backend.URL)
	if err != nil {
		t.Fatalf(
			"newHandlerForBackend failed: %v",
			err,
		)
	}

	gateway := httptest.NewServer(handler)
	defer gateway.Close()

	const workers = 10

	var wg sync.WaitGroup

	statuses := make(chan int, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			req, err := http.NewRequest(
				http.MethodPost,
				gateway.URL+"/api/orders",
				strings.NewReader(`{"customer":"varun"}`),
			)

			if err != nil {
				t.Errorf(
					"creating request failed: %v",
					err,
				)
				return
			}

			req.Header.Set(
				"Idempotency-Key",
				"concurrent-order",
			)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf(
					"request failed: %v",
					err,
				)
				return
			}

			statuses <- resp.StatusCode

			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	}

	wg.Wait()

	close(statuses)

	successes := 0
	conflicts := 0

	for status := range statuses {
		switch status {
		case http.StatusCreated:
			successes++

		case http.StatusConflict:
			conflicts++

		default:
			t.Fatalf(
				"unexpected status %d",
				status,
			)
		}
	}

	if successes != 1 {
		t.Fatalf(
			"expected exactly 1 successful request, got %d",
			successes,
		)
	}

	if conflicts != workers-1 {
		t.Fatalf(
			"expected %d conflicts, got %d",
			workers-1,
			conflicts,
		)
	}

	if backendCalls.Load() != 1 {
		t.Fatalf(
			"expected exactly 1 backend call, got %d",
			backendCalls.Load(),
		)
	}
}
