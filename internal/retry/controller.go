package retry

import (
	"context"
	"time"
)

type Controller struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

type AttemptResult struct {
	StatusCode int
	Err        error
}

func (c Controller) ShouldAttempt(attempt int) bool {
	return attempt >= 1 && attempt <= c.MaxAttempts
}

// Delay returns the delay AFTER an attempt fails.
//
// attempt 1 -> BaseDelay
// attempt 2 -> BaseDelay * 2
// attempt 3 -> BaseDelay * 4
func (c Controller) Delay(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}

	delay := c.BaseDelay

	for i := 1; i < attempt; i++ {
		delay *= 2
	}

	return delay
}

func (c Controller) Wait(
	ctx context.Context,
	attempt int,
) error {
	delay := c.Delay(attempt)

	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
