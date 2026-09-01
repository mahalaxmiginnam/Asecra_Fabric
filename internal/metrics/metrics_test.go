package metrics

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMetricsRecordRequest(t *testing.T) {
	m := New()

	m.RequestStarted()

	m.RecordRequest(
		http.MethodGet,
		"/api/orders",
		http.StatusOK,
		25*time.Millisecond,
	)

	m.RequestFinished()

	snapshot := m.Snapshot()

	if snapshot.RequestsInFlight != 0 {
		t.Fatalf(
			"expected zero requests in flight, got %d",
			snapshot.RequestsInFlight,
		)
	}

	if len(snapshot.RequestsTotal) != 1 {
		t.Fatalf(
			"expected one request counter, got %d",
			len(snapshot.RequestsTotal),
		)
	}
}

func TestMetricsRecordsFailures(t *testing.T) {
	m := New()

	m.RecordRequest(
		http.MethodGet,
		"/api/orders",
		http.StatusInternalServerError,
		time.Second,
	)

	snapshot := m.Snapshot()

	if len(snapshot.RequestsFailed) != 1 {
		t.Fatalf(
			"expected one failed request counter, got %d",
			len(snapshot.RequestsFailed),
		)
	}
}

func TestMetricsRecordsOperationalEvents(t *testing.T) {
	m := New()

	m.RecordUpstreamRequest()
	m.RecordUpstreamFailure()

	m.RecordRetry()
	m.RecordRetry()

	m.RecordCircuitBreakerRejection()

	snapshot := m.Snapshot()

	if snapshot.UpstreamRequestsTotal != 1 {
		t.Fatalf(
			"expected 1 upstream request, got %d",
			snapshot.UpstreamRequestsTotal,
		)
	}

	if snapshot.UpstreamFailuresTotal != 1 {
		t.Fatalf(
			"expected 1 upstream failure, got %d",
			snapshot.UpstreamFailuresTotal,
		)
	}

	if snapshot.RetriesTotal != 2 {
		t.Fatalf(
			"expected 2 retries, got %d",
			snapshot.RetriesTotal,
		)
	}

	if snapshot.CircuitBreakerRejections != 1 {
		t.Fatalf(
			"expected 1 circuit breaker rejection, got %d",
			snapshot.CircuitBreakerRejections,
		)
	}
}

func TestMetricsPrometheus(t *testing.T) {
	m := New()

	m.RecordRequest(
		http.MethodGet,
		"/api/orders",
		http.StatusOK,
		50*time.Millisecond,
	)

	output := m.Snapshot().Prometheus()

	required := []string{
		"asecra_gateway_requests_total",
		"asecra_gateway_requests_in_flight",
		"asecra_gateway_upstream_requests_total",
		"asecra_gateway_retries_total",
		"asecra_gateway_request_duration_seconds",
	}

	for _, metric := range required {
		if !strings.Contains(output, metric) {
			t.Fatalf(
				"expected metric %q in output:\n%s",
				metric,
				output,
			)
		}
	}
}

func TestMetricsConcurrentAccess(t *testing.T) {
	m := New()

	const workers = 20
	const iterations = 100

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				m.RequestStarted()

				m.RecordRequest(
					http.MethodGet,
					"/api/orders",
					http.StatusOK,
					time.Millisecond,
				)

				m.RecordUpstreamRequest()
				m.RecordRetry()

				m.RequestFinished()
			}
		}()
	}

	wg.Wait()

	snapshot := m.Snapshot()

	expected := uint64(workers * iterations)

	if snapshot.UpstreamRequestsTotal != expected {
		t.Fatalf(
			"expected %d upstream requests, got %d",
			expected,
			snapshot.UpstreamRequestsTotal,
		)
	}

	if snapshot.RetriesTotal != expected {
		t.Fatalf(
			"expected %d retries, got %d",
			expected,
			snapshot.RetriesTotal,
		)
	}

	if snapshot.RequestsInFlight != 0 {
		t.Fatalf(
			"expected zero requests in flight, got %d",
			snapshot.RequestsInFlight,
		)
	}
}
