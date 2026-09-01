package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu sync.RWMutex

	requestsTotal  map[string]uint64
	requestsFailed map[string]uint64

	upstreamRequestsTotal uint64
	upstreamFailuresTotal uint64

	retriesTotal             uint64
	circuitBreakerRejections uint64

	requestsInFlight int64

	requestDurations map[string]*durationStats
}

type durationStats struct {
	count uint64
	sum   time.Duration
}

func New() *Metrics {
	return &Metrics{
		requestsTotal:    make(map[string]uint64),
		requestsFailed:   make(map[string]uint64),
		requestDurations: make(map[string]*durationStats),
	}
}

func (m *Metrics) RequestStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestsInFlight++
}

func (m *Metrics) RequestFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.requestsInFlight > 0 {
		m.requestsInFlight--
	}
}

func (m *Metrics) RecordRequest(
	method string,
	route string,
	status int,
	duration time.Duration,
) {
	key := requestKey(method, route, status)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestsTotal[key]++

	if status >= http.StatusInternalServerError {
		m.requestsFailed[key]++
	}

	durationKey := routeKey(method, route)

	stats := m.requestDurations[durationKey]
	if stats == nil {
		stats = &durationStats{}
		m.requestDurations[durationKey] = stats
	}

	stats.count++
	stats.sum += duration
}

func (m *Metrics) RecordUpstreamRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.upstreamRequestsTotal++
}

func (m *Metrics) RecordUpstreamFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.upstreamFailuresTotal++
}

func (m *Metrics) RecordRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.retriesTotal++
}

func (m *Metrics) RecordCircuitBreakerRejection() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.circuitBreakerRejections++
}

func (m *Metrics) RequestsInFlight() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.requestsInFlight
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requestsTotal := copyMap(m.requestsTotal)
	requestsFailed := copyMap(m.requestsFailed)

	durations := make(map[string]DurationSnapshot, len(m.requestDurations))

	for key, stats := range m.requestDurations {
		durations[key] = DurationSnapshot{
			Count: stats.count,
			Sum:   stats.sum,
		}
	}

	return Snapshot{
		RequestsTotal:            requestsTotal,
		RequestsFailed:           requestsFailed,
		RequestsInFlight:         m.requestsInFlight,
		UpstreamRequestsTotal:    m.upstreamRequestsTotal,
		UpstreamFailuresTotal:    m.upstreamFailuresTotal,
		RetriesTotal:             m.retriesTotal,
		CircuitBreakerRejections: m.circuitBreakerRejections,
		RequestDurations:         durations,
	}
}

type Snapshot struct {
	RequestsTotal            map[string]uint64
	RequestsFailed           map[string]uint64
	RequestsInFlight         int64
	UpstreamRequestsTotal    uint64
	UpstreamFailuresTotal    uint64
	RetriesTotal             uint64
	CircuitBreakerRejections uint64
	RequestDurations         map[string]DurationSnapshot
}

type DurationSnapshot struct {
	Count uint64
	Sum   time.Duration
}

func (s Snapshot) Prometheus() string {
	var b strings.Builder

	writeCounterMap(
		&b,
		"asecra_gateway_requests_total",
		s.RequestsTotal,
	)

	writeCounterMap(
		&b,
		"asecra_gateway_requests_failed_total",
		s.RequestsFailed,
	)

	writeGauge(
		&b,
		"asecra_gateway_requests_in_flight",
		s.RequestsInFlight,
	)

	writeCounter(
		&b,
		"asecra_gateway_upstream_requests_total",
		s.UpstreamRequestsTotal,
	)

	writeCounter(
		&b,
		"asecra_gateway_upstream_failures_total",
		s.UpstreamFailuresTotal,
	)

	writeCounter(
		&b,
		"asecra_gateway_retries_total",
		s.RetriesTotal,
	)

	writeCounter(
		&b,
		"asecra_gateway_circuit_breaker_rejections_total",
		s.CircuitBreakerRejections,
	)

	writeDurationMap(
		&b,
		"asecra_gateway_request_duration_seconds",
		s.RequestDurations,
	)

	return b.String()
}

func Handler(m *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(
				w,
				"method not allowed",
				http.StatusMethodNotAllowed,
			)

			return
		}

		w.Header().Set(
			"Content-Type",
			"text/plain; version=0.0.4",
		)

		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(m.Snapshot().Prometheus()))
	})
}

func requestKey(
	method string,
	route string,
	status int,
) string {
	return fmt.Sprintf(
		`method="%s",route="%s",status="%d"`,
		escapeLabel(method),
		escapeLabel(route),
		status,
	)
}

func routeKey(
	method string,
	route string,
) string {
	return fmt.Sprintf(
		`method="%s",route="%s"`,
		escapeLabel(method),
		escapeLabel(route),
	)
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)

	return value
}

func copyMap(
	source map[string]uint64,
) map[string]uint64 {
	result := make(map[string]uint64, len(source))

	for key, value := range source {
		result[key] = value
	}

	return result
}

func writeCounter(
	b *strings.Builder,
	name string,
	value uint64,
) {
	fmt.Fprintf(
		b,
		"# TYPE %s counter\n%s %d\n",
		name,
		name,
		value,
	)
}

func writeGauge(
	b *strings.Builder,
	name string,
	value int64,
) {
	fmt.Fprintf(
		b,
		"# TYPE %s gauge\n%s %d\n",
		name,
		name,
		value,
	)
}

func writeCounterMap(
	b *strings.Builder,
	name string,
	values map[string]uint64,
) {
	fmt.Fprintf(
		b,
		"# TYPE %s counter\n",
		name,
	)

	keys := sortedKeys(values)

	for _, key := range keys {
		fmt.Fprintf(
			b,
			"%s{%s} %d\n",
			name,
			key,
			values[key],
		)
	}
}

func writeDurationMap(
	b *strings.Builder,
	name string,
	values map[string]DurationSnapshot,
) {
	fmt.Fprintf(
		b,
		"# TYPE %s_seconds summary\n",
		name,
	)

	keys := make([]string, 0, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		value := values[key]

		fmt.Fprintf(
			b,
			"%s_sum{%s} %.9f\n",
			name,
			key,
			value.Sum.Seconds(),
		)

		fmt.Fprintf(
			b,
			"%s_count{%s} %d\n",
			name,
			key,
			value.Count,
		)
	}
}

func sortedKeys(
	values map[string]uint64,
) []string {
	keys := make([]string, 0, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
