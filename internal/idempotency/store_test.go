package idempotency

import (
	"testing"
	"time"
)

func TestStoreCreateAndGet(t *testing.T) {
	store := NewStore(time.Minute)

	key := "test-key"

	if !store.Create(key) {
		t.Fatal("expected first Create to succeed")
	}

	entry, ok := store.Get(key)

	if !ok {
		t.Fatal("expected entry to exist")
	}

	if entry.Status != StatusInFlight {
		t.Fatalf(
			"expected status %q, got %q",
			StatusInFlight,
			entry.Status,
		)
	}
}

func TestStoreDuplicateCreate(t *testing.T) {
	store := NewStore(time.Minute)

	key := "duplicate-key"

	if !store.Create(key) {
		t.Fatal("expected first Create to succeed")
	}

	if store.Create(key) {
		t.Fatal("expected duplicate Create to fail")
	}
}

func TestStoreComplete(t *testing.T) {
	store := NewStore(time.Minute)

	key := "complete-key"

	if !store.Create(key) {
		t.Fatal("expected Create to succeed")
	}

	store.Complete(key, Response{
		StatusCode: 201,
		Header: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: []byte(`{"created":true}`),
	})

	entry, ok := store.Get(key)

	if !ok {
		t.Fatal("expected completed entry to exist")
	}

	if entry.Status != StatusComplete {
		t.Fatalf(
			"expected status %q, got %q",
			StatusComplete,
			entry.Status,
		)
	}

	if entry.Response == nil {
		t.Fatal("expected response")
	}

	if entry.Response.StatusCode != 201 {
		t.Fatalf(
			"expected status code 201, got %d",
			entry.Response.StatusCode,
		)
	}

	if string(entry.Response.Body) != `{"created":true}` {
		t.Fatalf(
			"unexpected body: %s",
			entry.Response.Body,
		)
	}
}

func TestStoreExpires(t *testing.T) {
	store := NewStore(50 * time.Millisecond)

	key := "expiry-key"

	if !store.Create(key) {
		t.Fatal("expected Create to succeed")
	}

	time.Sleep(75 * time.Millisecond)

	_, ok := store.Get(key)

	if ok {
		t.Fatal("expected entry to expire")
	}

	if !store.Create(key) {
		t.Fatal("expected Create to succeed after expiration")
	}
}

func TestStoreMissingComplete(t *testing.T) {
	store := NewStore(time.Minute)

	store.Complete(
		"missing-key",
		Response{
			StatusCode: 200,
			Body:       []byte("ok"),
		},
	)

	if _, ok := store.Get("missing-key"); ok {
		t.Fatal("expected missing entry to remain missing")
	}
}

func TestStoreConcurrentCreate(t *testing.T) {
	store := NewStore(time.Minute)

	key := "concurrent-key"

	const workers = 100

	results := make(chan bool, workers)

	for i := 0; i < workers; i++ {
		go func() {
			results <- store.Create(key)
		}()
	}

	successes := 0

	for i := 0; i < workers; i++ {
		if <-results {
			successes++
		}
	}

	if successes != 1 {
		t.Fatalf(
			"expected exactly one successful Create, got %d",
			successes,
		)
	}
}
