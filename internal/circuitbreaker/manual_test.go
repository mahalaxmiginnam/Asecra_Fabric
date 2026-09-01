package circuitbreaker

import (
	"testing"
	"time"
)

func TestManualStateTransition(t *testing.T) {
	b := New(3, 10*time.Second)

	if b.State() != Closed {
		t.Fatal("expected CLOSED")
	}

	b.Failure()

	if b.State() != Closed {
		t.Fatal("expected CLOSED after failure 1")
	}

	b.Failure()

	if b.State() != Closed {
		t.Fatal("expected CLOSED after failure 2")
	}

	b.Failure()

	if b.State() != Open {
		t.Fatal("expected OPEN after failure 3")
	}

	if err := b.Allow(); err != ErrOpen {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
}
