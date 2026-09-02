package proxy

import (
	"context"

	"net/http"
	"net/http/httptest"
	"net/url"

	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

func retryController() retry.Controller {
	return retry.Controller{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
	}
}

func TestExecutorCancelsSlowUpstream(t *testing.T) {
	upstreamStarted := make(chan struct{})
	upstreamCancelled := make(chan struct{})

	backend := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			close(upstreamStarted)

			<-r.Context().Done()

			close(upstreamCancelled)
		}),
	)

	defer backend.Close()

	upstream, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	executor := &Executor{
		Client: &http.Client{
			Timeout: 0,
		},
		Upstream: upstream,
		Retry: retry.Controller{
			MaxAttempts: 1,
			BaseDelay:   0,
		},
		RequestTimeout: 50 * time.Millisecond,
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/slow",
		nil,
	)

	result, err := executor.Execute(
		context.Background(),
		req,
	)

	if err == nil {
		t.Fatal("expected timeout error")
	}

	if result != nil {
		t.Fatal("expected nil result")
	}

	select {
	case <-upstreamStarted:
	case <-time.After(time.Second):
		t.Fatal("upstream request never started")
	}

	select {
	case <-upstreamCancelled:
	case <-time.After(time.Second):
		t.Fatal("upstream request was not cancelled")
	}
}

func TestExecutorStopsRetryWhenRequestTimeoutExpires(t *testing.T) {
	attempts := make(chan struct{}, 10)

	backend := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			attempts <- struct{}{}

			http.Error(
				w,
				"service unavailable",
				http.StatusServiceUnavailable,
			)
		}),
	)

	defer backend.Close()

	upstream, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	executor := &Executor{
		Client: &http.Client{
			Timeout: 0,
		},
		Upstream: upstream,
		Retry: retry.Controller{
			MaxAttempts: 10,
			BaseDelay:   100 * time.Millisecond,
		},
		RequestTimeout: 50 * time.Millisecond,
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/retry",
		nil,
	)

	result, err := executor.Execute(
		context.Background(),
		req,
	)

	if err == nil {
		t.Fatal("expected timeout error")
	}

	if result != nil {
		t.Fatal("expected nil result")
	}

	if len(attempts) >= 10 {
		t.Fatal("expected request timeout to stop retries")
	}
}

func TestResultWriteToRemovesContentLength(t *testing.T) {
	result := &Result{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Content-Length": []string{"999"},
		},
		Body: []byte(`{"ok":true}`),
	}

	recorder := httptest.NewRecorder()

	err := result.WriteTo(recorder)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if recorder.Header().Get("Content-Length") != "" {
		t.Fatalf(
			"expected Content-Length to be removed, got %q",
			recorder.Header().Get("Content-Length"),
		)
	}

	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf(
			"expected Content-Type to be preserved, got %q",
			recorder.Header().Get("Content-Type"),
		)
	}

	if recorder.Body.String() != `{"ok":true}` {
		t.Fatalf(
			"unexpected response body: %q",
			recorder.Body.String(),
		)
	}
}

func TestExecutorDoesNotForwardIncomingHostAsHeader(t *testing.T) {
	var receivedHost string

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHost = r.Host
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream URL: %v", err)
	}

	executor := &Executor{
		Client:         upstream.Client(),
		Upstream:       upstreamURL,
		Retry:          retryController(),
		RequestTimeout: time.Second,
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
		nil,
	)
	req.Host = "gateway.local"

	_, err = executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if receivedHost != upstreamURL.Host {
		t.Fatalf(
			"upstream Host = %q, want %q",
			receivedHost,
			upstreamURL.Host,
		)
	}
}
func TestExecutorDoesNotForwardConnectionHeader(t *testing.T) {
	var receivedConnection string

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedConnection = r.Header.Get("Connection")
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream URL: %v", err)
	}

	executor := &Executor{
		Client:         upstream.Client(),
		Upstream:       upstreamURL,
		Retry:          retryController(),
		RequestTimeout: time.Second,
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
		nil,
	)
	req.Header.Set("Connection", "keep-alive")

	_, err = executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if receivedConnection != "" {
		t.Fatalf(
			"upstream Connection header = %q, want empty",
			receivedConnection,
		)
	}
}

func TestExecutorDoesNotForwardHopByHopHeaders(t *testing.T) {
	var received http.Header

	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("failed to parse upstream URL: %v", err)
	}

	executor := &Executor{
		Client:         upstream.Client(),
		Upstream:       upstreamURL,
		Retry:          retryController(),
		RequestTimeout: time.Second,
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/api/test",
		nil,
	)

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Upgrade", "websocket")

	_, err = executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	for _, name := range []string{
		"Connection",
		"Keep-Alive",
		"Upgrade",
	} {
		if value := received.Get(name); value != "" {
			t.Fatalf(
				"upstream %s header = %q, want empty",
				name,
				value,
			)
		}
	}
}
