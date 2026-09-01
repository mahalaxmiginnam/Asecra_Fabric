package idempotency

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMiddlewareReplaysCompletedResponse(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	var executions atomic.Int32

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executions.Add(1)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"created":true}`))
		}),
	)

	first := httptest.NewRecorder()

	req1 := httptest.NewRequest(
		http.MethodPost,
		"/orders",
		nil,
	)
	req1.Header.Set(HeaderIdempotencyKey, "order-123")

	handler.ServeHTTP(first, req1)

	if first.Code != http.StatusCreated {
		t.Fatalf(
			"expected first status 201, got %d",
			first.Code,
		)
	}

	if first.Body.String() != `{"created":true}` {
		t.Fatalf(
			"unexpected first response: %s",
			first.Body.String(),
		)
	}

	second := httptest.NewRecorder()

	req2 := httptest.NewRequest(
		http.MethodPost,
		"/orders",
		nil,
	)
	req2.Header.Set(HeaderIdempotencyKey, "order-123")

	handler.ServeHTTP(second, req2)

	if second.Code != http.StatusCreated {
		t.Fatalf(
			"expected replay status 201, got %d",
			second.Code,
		)
	}

	if second.Body.String() != `{"created":true}` {
		t.Fatalf(
			"unexpected replay response: %s",
			second.Body.String(),
		)
	}

	if executions.Load() != 1 {
		t.Fatalf(
			"expected exactly one downstream execution, got %d",
			executions.Load(),
		)
	}

	if second.Header().Get("Content-Type") != "application/json" {
		t.Fatalf(
			"expected replayed Content-Type header, got %q",
			second.Header().Get("Content-Type"),
		)
	}
}

func TestMiddlewareRejectsConcurrentRequest(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	started := make(chan struct{})
	release := make(chan struct{})

	var executions atomic.Int32

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executions.Add(1)

			close(started)
			<-release

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		}),
	)

	firstDone := make(chan struct{})

	firstResponse := httptest.NewRecorder()

	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/orders",
		nil,
	)
	firstRequest.Header.Set(HeaderIdempotencyKey, "concurrent-123")

	go func() {
		handler.ServeHTTP(firstResponse, firstRequest)
		close(firstDone)
	}()

	<-started

	secondResponse := httptest.NewRecorder()

	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/orders",
		nil,
	)
	secondRequest.Header.Set(HeaderIdempotencyKey, "concurrent-123")

	handler.ServeHTTP(secondResponse, secondRequest)

	if secondResponse.Code != http.StatusConflict {
		t.Fatalf(
			"expected concurrent request status 409, got %d",
			secondResponse.Code,
		)
	}

	close(release)
	<-firstDone

	if firstResponse.Code != http.StatusCreated {
		t.Fatalf(
			"expected first request status 201, got %d",
			firstResponse.Code,
		)
	}

	if executions.Load() != 1 {
		t.Fatalf(
			"expected exactly one downstream execution, got %d",
			executions.Load(),
		)
	}
}

func TestMiddlewareIgnoresIdempotencyForGet(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	var executions atomic.Int32

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executions.Add(1)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()

		req := httptest.NewRequest(
			http.MethodGet,
			"/orders",
			nil,
		)

		req.Header.Set(
			HeaderIdempotencyKey,
			"get-key",
		)

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"request %d: expected 200, got %d",
				i+1,
				recorder.Code,
			)
		}
	}

	if executions.Load() != 2 {
		t.Fatalf(
			"expected GET to execute twice, got %d executions",
			executions.Load(),
		)
	}
}

func TestMiddlewareProtectsStateChangingMethods(t *testing.T) {
	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			store := NewStore(time.Minute)
			controller := NewController(store)
			middleware := NewMiddleware(controller)

			var executions atomic.Int32

			handler := middleware.Handler(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					executions.Add(1)

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("ok"))
				}),
			)

			for i := 0; i < 2; i++ {
				recorder := httptest.NewRecorder()

				req := httptest.NewRequest(
					method,
					"/resource",
					nil,
				)

				req.Header.Set(
					HeaderIdempotencyKey,
					"method-key",
				)

				handler.ServeHTTP(recorder, req)

				if recorder.Code != http.StatusOK {
					t.Fatalf(
						"request %d: expected 200, got %d",
						i+1,
						recorder.Code,
					)
				}
			}

			if executions.Load() != 1 {
				t.Fatalf(
					"expected one execution for %s, got %d",
					method,
					executions.Load(),
				)
			}
		})
	}
}

func TestMiddlewareWithoutKeyExecutesNormally(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	var executions atomic.Int32

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executions.Add(1)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()

		req := httptest.NewRequest(
			http.MethodPost,
			"/orders",
			nil,
		)

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf(
				"request %d: expected 200, got %d",
				i+1,
				recorder.Code,
			)
		}
	}

	if executions.Load() != 2 {
		t.Fatalf(
			"expected two executions without key, got %d",
			executions.Load(),
		)
	}
}

func TestMiddlewareReplaysMultipleHeaders(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Test", "one")
			w.Header().Add("X-Test", "two")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	)

	first := httptest.NewRecorder()

	req1 := httptest.NewRequest(
		http.MethodPost,
		"/resource",
		nil,
	)
	req1.Header.Set(HeaderIdempotencyKey, "headers-key")

	handler.ServeHTTP(first, req1)

	second := httptest.NewRecorder()

	req2 := httptest.NewRequest(
		http.MethodPost,
		"/resource",
		nil,
	)
	req2.Header.Set(HeaderIdempotencyKey, "headers-key")

	handler.ServeHTTP(second, req2)

	values := second.Header().Values("X-Test")

	if len(values) != 2 {
		t.Fatalf(
			"expected two X-Test values, got %d",
			len(values),
		)
	}

	if values[0] != "one" || values[1] != "two" {
		t.Fatalf(
			"unexpected X-Test values: %#v",
			values,
		)
	}
}

func TestMiddlewareDoesNotReplayStaleContentLength(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "99999")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello"))
		}),
	)

	first := httptest.NewRecorder()

	req1 := httptest.NewRequest(
		http.MethodPost,
		"/resource",
		nil,
	)
	req1.Header.Set(HeaderIdempotencyKey, "length-key")

	handler.ServeHTTP(first, req1)

	second := httptest.NewRecorder()

	req2 := httptest.NewRequest(
		http.MethodPost,
		"/resource",
		nil,
	)
	req2.Header.Set(HeaderIdempotencyKey, "length-key")

	handler.ServeHTTP(second, req2)

	if second.Header().Get("Content-Length") == "99999" {
		t.Fatal("replay contains stale Content-Length")
	}

	if second.Body.String() != "hello" {
		t.Fatalf(
			"unexpected replay body: %q",
			second.Body.String(),
		)
	}
}

func TestMiddlewareConcurrentSameKeyOnlyOneExecutes(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)
	middleware := NewMiddleware(controller)

	const workers = 50

	var executions atomic.Int32

	start := make(chan struct{})
	release := make(chan struct{})

	handler := middleware.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executions.Add(1)

			<-release

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		}),
	)

	var wg sync.WaitGroup
	wg.Add(workers)

	responses := make([]*httptest.ResponseRecorder, workers)

	for i := 0; i < workers; i++ {
		responses[i] = httptest.NewRecorder()

		go func(index int) {
			defer wg.Done()

			<-start

			req := httptest.NewRequest(
				http.MethodPost,
				"/orders",
				nil,
			)

			req.Header.Set(
				HeaderIdempotencyKey,
				"same-key",
			)

			handler.ServeHTTP(
				responses[index],
				req,
			)
		}(i)
	}

	close(start)

	time.Sleep(10 * time.Millisecond)

	if executions.Load() != 1 {
		t.Fatalf(
			"expected one active execution, got %d",
			executions.Load(),
		)
	}

	close(release)
	wg.Wait()

	successes := 0
	conflicts := 0

	for _, response := range responses {
		switch response.Code {
		case http.StatusCreated:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf(
				"unexpected response status %d",
				response.Code,
			)
		}
	}

	if successes != 1 {
		t.Fatalf(
			"expected exactly one successful execution, got %d",
			successes,
		)
	}

	if conflicts != workers-1 {
		t.Fatalf(
			"expected %d conflicts, got %d",
			workers-1,
			conflicts,
		)
	}
}
