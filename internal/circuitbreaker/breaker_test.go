package circuitbreaker

import (
	"testing"
	"time"
)

func TestBreakerStartsClosed(t *testing.T) {

	b := New(3, time.Second)

	if b.State() != Closed {
		t.Fatalf(
			"expected CLOSED, got %v",
			b.State(),
		)
	}

	if err := b.Allow(); err != nil {
		t.Fatalf(
			"expected request to be allowed, got %v",
			err,
		)
	}
}

func TestBreakerOpensAfterThreshold(t *testing.T) {

	b := New(3, time.Second)

	b.Failure()
	b.Failure()

	if b.State() != Closed {
		t.Fatal("breaker opened too early")
	}

	b.Failure()

	if b.State() != Open {
		t.Fatalf(
			"expected OPEN, got %v",
			b.State(),
		)
	}

	if err := b.Allow(); err != ErrOpen {
		t.Fatalf(
			"expected ErrOpen, got %v",
			err,
		)
	}
}

func TestBreakerTransitionsToHalfOpen(t *testing.T) {

	b := New(1, 50*time.Millisecond)

	b.Failure()

	if b.State() != Open {
		t.Fatal("expected OPEN")
	}

	time.Sleep(70 * time.Millisecond)

	if b.State() != HalfOpen {
		t.Fatalf(
			"expected HALF_OPEN, got %v",
			b.State(),
		)
	}

	if err := b.Allow(); err != nil {
		t.Fatalf(
			"expected probe to be allowed, got %v",
			err,
		)
	}
}

func TestHalfOpenProbeIsExclusive(t *testing.T) {

	b := New(1, 20*time.Millisecond)

	b.Failure()

	time.Sleep(30 * time.Millisecond)

	if err := b.Allow(); err != nil {
		t.Fatalf(
			"expected first probe to be allowed, got %v",
			err,
		)
	}

	if err := b.Allow(); err != ErrBusy {
		t.Fatalf(
			"expected ErrBusy, got %v",
			err,
		)
	}
}

func TestSuccessfulProbeClosesBreaker(t *testing.T) {

	b := New(1, 20*time.Millisecond)

	b.Failure()

	time.Sleep(30 * time.Millisecond)

	if err := b.Allow(); err != nil {
		t.Fatal(err)
	}

	b.Success()

	if b.State() != Closed {
		t.Fatalf(
			"expected CLOSED, got %v",
			b.State(),
		)
	}

	if err := b.Allow(); err != nil {
		t.Fatalf(
			"expected request to be allowed, got %v",
			err,
		)
	}
}

func TestFailedProbeReopensBreaker(t *testing.T) {

	b := New(1, 20*time.Millisecond)

	b.Failure()

	time.Sleep(30 * time.Millisecond)

	if err := b.Allow(); err != nil {
		t.Fatal(err)
	}

	b.Failure()

	if b.State() != Open {
		t.Fatalf(
			"expected OPEN, got %v",
			b.State(),
		)
	}
}
