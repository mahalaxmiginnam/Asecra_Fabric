package retry

import (
	"net/http"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		statusCode int
		expected   Decision
	}{
		{
			name:       "GET 500 is retryable",
			method:     http.MethodGet,
			statusCode: http.StatusInternalServerError,
			expected:   Retry,
		},
		{
			name:       "GET 503 is retryable",
			method:     http.MethodGet,
			statusCode: http.StatusServiceUnavailable,
			expected:   Retry,
		},
		{
			name:       "GET 429 is retryable",
			method:     http.MethodGet,
			statusCode: http.StatusTooManyRequests,
			expected:   Retry,
		},
		{
			name:       "GET 404 is not retryable",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			expected:   DoNotRetry,
		},
		{
			name:       "GET 401 is not retryable",
			method:     http.MethodGet,
			statusCode: http.StatusUnauthorized,
			expected:   DoNotRetry,
		},
		{
			name:       "POST 500 is not retryable",
			method:     http.MethodPost,
			statusCode: http.StatusInternalServerError,
			expected:   DoNotRetry,
		},
		{
			name:       "PATCH 503 is not retryable",
			method:     http.MethodPatch,
			statusCode: http.StatusServiceUnavailable,
			expected:   DoNotRetry,
		},
		{
			name:       "GET 200 is not retryable",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			expected:   DoNotRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsRetryable(
				RequestPolicy{
					Method: tt.method,
				},
				ResponsePolicy{
					StatusCode: tt.statusCode,
				},
			)

			if actual != tt.expected {
				t.Fatalf(
					"expected %v, got %v",
					tt.expected,
					actual,
				)
			}
		})
	}
}
