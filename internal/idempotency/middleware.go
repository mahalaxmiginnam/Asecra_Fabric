package idempotency

import (
	"bytes"
	"io"
	"net/http"
)

const HeaderIdempotencyKey = "Idempotency-Key"

type Middleware struct {
	Controller *Controller
}

func NewMiddleware(controller *Controller) *Middleware {
	return &Middleware{
		Controller: controller,
	}
}

// Handler is the gateway middleware entry point.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		// Idempotency applies to state-changing HTTP methods.
		switch r.Method {
		case http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get(HeaderIdempotencyKey)

		decision, replay, err := m.Controller.Begin(key)

		if err != nil {
			if err == ErrInFlight {
				http.Error(
					w,
					"idempotent request already in progress",
					http.StatusConflict,
				)
				return
			}

			http.Error(
				w,
				"idempotency error",
				http.StatusInternalServerError,
			)
			return
		}

		// Existing completed request.
		if decision == Replay {
			writeResponse(w, replay)
			return
		}

		// No idempotency key.
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Capture downstream response.
		recorder := newResponseRecorder()

		next.ServeHTTP(recorder, r)

		response := Response{
			StatusCode: recorder.statusCode,
			Header:     recorder.header.Clone(),
			Body: append(
				[]byte(nil),
				recorder.body.Bytes()...,
			),
		}

		// Store response for future replay.
		m.Controller.Complete(
			key,
			response,
		)

		// Return response to client.
		writeResponse(
			w,
			&response,
		)
	})
}

// Wrap is retained as an alias.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return m.Handler(next)
}

type responseRecorder struct {
	header      http.Header
	body        bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(
	statusCode int,
) {
	if r.wroteHeader {
		return
	}

	r.statusCode = statusCode
	r.wroteHeader = true
}

func (r *responseRecorder) Write(
	data []byte,
) (int, error) {

	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	return r.body.Write(data)
}

func writeResponse(
	w http.ResponseWriter,
	response *Response,
) {
	if response == nil {
		http.Error(
			w,
			"missing idempotent response",
			http.StatusInternalServerError,
		)
		return
	}

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(
				key,
				value,
			)
		}
	}

	// Never replay a stale Content-Length header.
	w.Header().Del("Content-Length")

	w.WriteHeader(
		response.StatusCode,
	)

	_, _ = io.Copy(
		w,
		bytes.NewReader(response.Body),
	)
}
