package recovery

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareRecoversPanic(t *testing.T) {
	handler := Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}),
	)

	recorder := httptest.NewRecorder()

	req := httptest.NewRequest(
		http.MethodGet,
		"http://gateway.local/test",
		nil,
	)

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}
}
