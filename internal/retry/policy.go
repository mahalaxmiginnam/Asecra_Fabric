package retry

import "net/http"

type Decision int

const (
	DoNotRetry Decision = iota
	Retry
)

type RequestPolicy struct {
	Method string
}

type ResponsePolicy struct {
	StatusCode int
}

func IsRetryable(
	req RequestPolicy,
	resp ResponsePolicy,
) Decision {

	// POST and PATCH may have side effects.
	// We will handle safe retries for these later
	// using idempotency.
	if req.Method == http.MethodPost ||
		req.Method == http.MethodPatch {
		return DoNotRetry
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:

		return Retry

	default:
		return DoNotRetry
	}
}
