package metrics

import (
	"net/http"
	"time"
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
	m *Metrics,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		start := time.Now()

		m.RequestStarted()
		defer m.RequestFinished()

		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		m.RecordRequest(
			r.Method,
			r.URL.Path,
			recorder.statusCode,
			time.Since(start),
		)
	})
}
