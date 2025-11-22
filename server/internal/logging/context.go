package logging

import (
	"context"

	"go.uber.org/zap"
)

type contextKey string

const (
	loggerKey contextKey = "logger"
)

// WithLogger stores a logger in the context.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves a logger from the context.
// If no logger is found, it returns a no-op logger.
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop()
}

// AddFields adds fields to the logger stored in the context.
// Returns a new context with the updated logger.
func AddFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := FromContext(ctx)
	return WithLogger(ctx, logger.With(fields...))
}

// With adds fields to the logger and returns the updated logger.
// This is a convenience function for adding contextual information.
func With(ctx context.Context, fields ...zap.Field) *zap.Logger {
	return FromContext(ctx).With(fields...)
}

// Debug logs a debug message using the logger from context.
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Debug(msg, fields...)
}

// Info logs an info message using the logger from context.
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Info(msg, fields...)
}

// Warn logs a warning message using the logger from context.
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Warn(msg, fields...)
}

// Error logs an error message using the logger from context.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	FromContext(ctx).Error(msg, fields...)
}
