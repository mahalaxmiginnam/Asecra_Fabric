package idempotency

import (
	"testing"
	"time"
)

func TestControllerBeginAndComplete(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)

	key := "request-123"

	decision, response, err := controller.Begin(key)
	if err != nil {
		t.Fatalf("first Begin failed: %v", err)
	}

	if decision != Execute {
		t.Fatalf("expected Execute, got %v", decision)
	}

	if response != nil {
		t.Fatal("first request should not have a replay response")
	}

	controller.Complete(
		key,
		Response{
			StatusCode: 200,
			Header: map[string][]string{
				"Content-Type": {"application/json"},
			},
			Body: []byte(`{"ok":true}`),
		},
	)

	decision, response, err = controller.Begin(key)
	if err != nil {
		t.Fatalf("second Begin failed: %v", err)
	}

	if decision != Replay {
		t.Fatalf("expected Replay, got %v", decision)
	}

	if response == nil {
		t.Fatal("expected replay response")
	}

	if response.StatusCode != 200 {
		t.Fatalf(
			"expected status 200, got %d",
			response.StatusCode,
		)
	}

	if string(response.Body) != `{"ok":true}` {
		t.Fatalf(
			"unexpected body: %s",
			response.Body,
		)
	}
}

func TestControllerRejectsConcurrentRequest(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)

	key := "request-456"

	decision, _, err := controller.Begin(key)
	if err != nil {
		t.Fatalf("first Begin failed: %v", err)
	}

	if decision != Execute {
		t.Fatalf("expected Execute, got %v", decision)
	}

	decision, _, err = controller.Begin(key)

	if decision != Reject {
		t.Fatalf("expected Reject, got %v", decision)
	}

	if err != ErrInFlight {
		t.Fatalf(
			"expected ErrInFlight, got %v",
			err,
		)
	}
}

func TestControllerNoKeyExecutes(t *testing.T) {
	store := NewStore(time.Minute)
	controller := NewController(store)

	decision, response, err := controller.Begin("")

	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if decision != Execute {
		t.Fatalf(
			"expected Execute, got %v",
			decision,
		)
	}

	if response != nil {
		t.Fatal("expected nil response")
	}
}

func TestControllerExpiration(t *testing.T) {
	store := NewStore(20 * time.Millisecond)
	controller := NewController(store)

	key := "request-expire"

	decision, _, err := controller.Begin(key)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if decision != Execute {
		t.Fatalf("expected Execute, got %v", decision)
	}

	controller.Complete(
		key,
		Response{
			StatusCode: 201,
			Body:       []byte("created"),
		},
	)

	time.Sleep(30 * time.Millisecond)

	decision, response, err := controller.Begin(key)
	if err != nil {
		t.Fatalf(
			"Begin after expiration failed: %v",
			err,
		)
	}

	if decision != Execute {
		t.Fatalf(
			"expected Execute after expiration, got %v",
			decision,
		)
	}

	if response != nil {
		t.Fatal("expired request should not replay")
	}
}
