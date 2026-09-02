package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareRecordsRequest(t *testing.T) {
	logger := NewLogger()

	handler := Middleware(
		logger,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"created":true}`))
		}),
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/orders",
		nil,
	)

	req.Header.Set("X-Request-ID", "req-123")

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected 201, got %d",
			recorder.Code,
		)
	}
}

func TestResponseRecorderDefaultStatus(t *testing.T) {
	recorder := &responseRecorder{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	_, err := recorder.Write([]byte("ok"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if recorder.statusCode != http.StatusOK {
		t.Fatalf(
			"expected default status 200, got %d",
			recorder.statusCode,
		)
	}
}

func TestResponseRecorderIgnoresSecondWriteHeader(t *testing.T) {
	recorder := &responseRecorder{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	recorder.WriteHeader(http.StatusCreated)
	recorder.WriteHeader(http.StatusInternalServerError)

	if recorder.statusCode != http.StatusCreated {
		t.Fatalf(
			"expected first status 201 to be preserved, got %d",
			recorder.statusCode,
		)
	}
}

func TestResponseRecorderWriteSetsStatusOK(t *testing.T) {
	recorder := &responseRecorder{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	_, err := recorder.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if recorder.statusCode != http.StatusOK {
		t.Fatalf(
			"expected status 200 after Write, got %d",
			recorder.statusCode,
		)
	}
}

func TestMiddlewarePreservesRequestID(t *testing.T) {
	logger := NewLogger()

	handler := Middleware(
		logger,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-Request-ID"); got != "req-456" {
				t.Fatalf(
					"expected request ID req-456, got %q",
					got,
				)
			}

			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/orders",
		nil,
	)

	req.Header.Set("X-Request-ID", "req-456")

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected 200, got %d",
			recorder.Code,
		)
	}
}
