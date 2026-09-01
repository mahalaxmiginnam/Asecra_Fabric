package observability

import (
	"net/http"
	"strconv"
	"time"
)

const (
	RequestIDHeader        = "X-Request-ID"
	UpstreamAttemptsHeader = "X-Upstream-Attempts"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}

	r.statusCode = statusCode
	r.wroteHeader = true

	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	return r.ResponseWriter.Write(body)
}

func Middleware(
	logger *Logger,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		start := time.Now()

		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		attempts := 0

		if value := r.Header.Get(UpstreamAttemptsHeader); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil {
				attempts = parsed
			}
		}

		logger.Request(
			r.Header.Get(RequestIDHeader),
			r.Method,
			r.URL.Path,
			recorder.statusCode,
			time.Since(start),
			attempts,
		)
	})
}
