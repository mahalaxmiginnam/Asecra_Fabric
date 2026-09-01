package retry

import (
	"context"
	"testing"
	"time"
)

func TestControllerShouldAttempt(t *testing.T) {

	controller := Controller{
		MaxAttempts: 3,
	}

	if !controller.ShouldAttempt(1) {
		t.Fatal("attempt 1 should be allowed")
	}

	if !controller.ShouldAttempt(2) {
		t.Fatal("attempt 2 should be allowed")
	}

	if !controller.ShouldAttempt(3) {
		t.Fatal("attempt 3 should be allowed")
	}

	if controller.ShouldAttempt(4) {
		t.Fatal("attempt 4 should not be allowed")
	}
}

func TestControllerDelay(t *testing.T) {

	controller := Controller{
		BaseDelay: 100 * time.Millisecond,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{
			attempt:  1,
			expected: 100 * time.Millisecond,
		},
		{
			attempt:  2,
			expected: 200 * time.Millisecond,
		},
		{
			attempt:  3,
			expected: 400 * time.Millisecond,
		},
	}

	for _, test := range tests {

		actual := controller.Delay(test.attempt)

		if actual != test.expected {
			t.Fatalf(
				"attempt %d: expected %v, got %v",
				test.attempt,
				test.expected,
				actual,
			)
		}
	}
}

func TestControllerWaitHonorsContext(t *testing.T) {

	controller := Controller{
		BaseDelay: 1 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	err := controller.Wait(ctx, 1)

	if err != context.Canceled {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}
