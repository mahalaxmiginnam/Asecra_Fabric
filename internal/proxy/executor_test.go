package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

func TestExecutorForwardsRequestBody(t *testing.T) {
	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}

			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}

			if string(body) != `{"customer":"varun"}` {
				t.Fatalf("unexpected body: %s", body)
			}

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"created":true}`))
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
		Upstream:       upstream,
		Retry:          retryController(),
		RequestTimeout: time.Second,
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/orders",
		strings.NewReader(`{"customer":"varun"}`),
	)

	result, err := executor.Execute(
		context.Background(),
		req,
	)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.StatusCode != http.StatusCreated {
		t.Fatalf(
			"expected 201, got %d",
			result.StatusCode,
		)
	}

	if string(result.Body) != `{"created":true}` {
		t.Fatalf(
			"unexpected response body: %s",
			result.Body,
		)
	}
}

func retryController() retry.Controller {
	return retry.Controller{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
	}
}
