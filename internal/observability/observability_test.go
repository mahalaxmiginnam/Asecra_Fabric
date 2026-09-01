package observability

import (
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger()

	if logger == nil {
		t.Fatal("expected logger, got nil")
	}

	if logger.logger == nil {
		t.Fatal("expected underlying slog logger, got nil")
	}
}

func TestLoggerRequest(t *testing.T) {
	logger := NewLogger()

	logger.Request(
		"req-123",
		"POST",
		"/api/orders",
		201,
		25*time.Millisecond,
		1,
	)
}
