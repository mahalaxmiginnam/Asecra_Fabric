package observability

import (
	"log/slog"
	"os"
	"time"
)

type Logger struct {
	logger *slog.Logger
}

func NewLogger() *Logger {
	handler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
	)

	return &Logger{
		logger: slog.New(handler),
	}
}

func (l *Logger) Info(
	msg string,
	args ...any,
) {
	l.logger.Info(msg, args...)
}

func (l *Logger) Error(
	msg string,
	args ...any,
) {
	l.logger.Error(msg, args...)
}

func (l *Logger) Request(
	requestID string,
	method string,
	path string,
	status int,
	duration time.Duration,
	attempts int,
) {
	l.logger.Info(
		"gateway request",
		"request_id", requestID,
		"method", method,
		"path", path,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"attempts", attempts,
	)
}
