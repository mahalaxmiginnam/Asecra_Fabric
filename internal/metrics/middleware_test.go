package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareRecordsRequest(t *testing.T) {
	metricsCollector := New()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	handler := Middleware(metricsCollector, next)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/orders",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			recorder.Code,
		)
	}

	snapshot := metricsCollector.Snapshot()

	if snapshot.RequestsInFlight != 0 {
		t.Fatalf(
			"expected zero in-flight requests, got %d",
			snapshot.RequestsInFlight,
		)
	}

	key := `method="POST",route="/api/orders",status="201"`

	if snapshot.RequestsTotal[key] != 1 {
		t.Fatalf(
			"expected one recorded request, got %d",
			snapshot.RequestsTotal[key],
		)
	}

	durationKey := `method="POST",route="/api/orders"`

	duration, ok := snapshot.RequestDurations[durationKey]
	if !ok {
		t.Fatalf("expected duration metrics for %s", durationKey)
	}

	if duration.Count != 1 {
		t.Fatalf(
			"expected duration count 1, got %d",
			duration.Count,
		)
	}

	if duration.Sum <= 0 {
		t.Fatalf(
			"expected positive duration, got %s",
			duration.Sum,
		)
	}

	output := snapshot.Prometheus()

	expectedMetrics := []string{
		`asecra_gateway_requests_total{` + key + `} 1`,
		`asecra_gateway_requests_in_flight 0`,
		`asecra_gateway_request_duration_seconds_count{` + durationKey + `} 1`,
	}

	for _, expected := range expectedMetrics {
		if !strings.Contains(output, expected) {
			t.Fatalf(
				"expected Prometheus output to contain %q, got:\n%s",
				expected,
				output,
			)
		}
	}

}
