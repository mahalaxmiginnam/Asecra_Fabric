package circuitbreaker

import "net/http"

func ShouldCountAsFailure(statusCode int) bool {
	switch {
	case statusCode >= 500:
		return true

	case statusCode == http.StatusTooManyRequests:
		return true

	default:
		return false
	}
}
