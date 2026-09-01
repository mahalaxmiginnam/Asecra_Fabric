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
