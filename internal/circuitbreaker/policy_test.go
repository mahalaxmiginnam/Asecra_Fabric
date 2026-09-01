package circuitbreaker

import (
	"net/http"
	"testing"
)

func TestShouldCountAsFailure(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{
			name:       "200 is not failure",
			statusCode: http.StatusOK,
			want:       false,
		},
		{
			name:       "400 is not failure",
			statusCode: http.StatusBadRequest,
			want:       false,
		},
		{
			name:       "401 is not failure",
			statusCode: http.StatusUnauthorized,
			want:       false,
		},
		{
			name:       "404 is not failure",
			statusCode: http.StatusNotFound,
			want:       false,
		},
		{
			name:       "429 is failure",
			statusCode: http.StatusTooManyRequests,
			want:       true,
		},
		{
			name:       "500 is failure",
			statusCode: http.StatusInternalServerError,
			want:       true,
		},
		{
			name:       "503 is failure",
			statusCode: http.StatusServiceUnavailable,
			want:       true,
		},
		{
			name:       "504 is failure",
			statusCode: http.StatusGatewayTimeout,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCountAsFailure(tt.statusCode)

			if got != tt.want {
				t.Fatalf(
					"expected %v, got %v",
					tt.want,
					got,
				)
			}
		})
	}
}
