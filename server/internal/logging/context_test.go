package logging

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestWithLogger(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := context.Background()

	ctx = WithLogger(ctx, logger)

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("Expected non-nil logger from context")
	}
}

func TestFromContext_NoLogger(t *testing.T) {
	ctx := context.Background()

	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("Expected no-op logger when none exists in context")
	}

	// Should not panic
	logger.Info("test message")
}

func TestFromContext_WithLogger(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("Expected non-nil logger from context")
	}
}

func TestAddFields(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	ctx = AddFields(ctx, zap.String("key", "value"))

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("Expected non-nil logger from context after adding fields")
	}

	// Should not panic
	retrieved.Info("test message")
}

func TestAddFields_NoLogger(t *testing.T) {
	ctx := context.Background()

	// Should not panic even without logger in context
	ctx = AddFields(ctx, zap.String("key", "value"))

	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("Expected no-op logger")
	}
}

func TestWith(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	newLogger := With(ctx, zap.String("key", "value"))
	if newLogger == nil {
		t.Fatal("Expected non-nil logger from With")
	}

	// Should not panic
	newLogger.Info("test message")
}

func TestDebug(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	// Should not panic
	Debug(ctx, "test debug message")
	Debug(ctx, "test with fields", zap.String("key", "value"))
}

func TestDebug_NoLogger(t *testing.T) {
	ctx := context.Background()

	// Should not panic even without logger
	Debug(ctx, "test message")
}

func TestInfo(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	// Should not panic
	Info(ctx, "test info message")
	Info(ctx, "test with fields", zap.String("key", "value"))
}

func TestInfo_NoLogger(t *testing.T) {
	ctx := context.Background()

	// Should not panic even without logger
	Info(ctx, "test message")
}

func TestWarn(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	// Should not panic
	Warn(ctx, "test warn message")
	Warn(ctx, "test with fields", zap.String("key", "value"))
}

func TestWarn_NoLogger(t *testing.T) {
	ctx := context.Background()

	// Should not panic even without logger
	Warn(ctx, "test message")
}

func TestError(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := WithLogger(context.Background(), logger)

	// Should not panic
	Error(ctx, "test error message")
	Error(ctx, "test with fields", zap.String("key", "value"))
}

func TestError_NoLogger(t *testing.T) {
	ctx := context.Background()

	// Should not panic even without logger
	Error(ctx, "test message")
}

func TestContextChaining(t *testing.T) {
	logger, _ := NewDevelopmentLogger()
	ctx := context.Background()

	// Chain multiple operations
	ctx = WithLogger(ctx, logger)
	ctx = AddFields(ctx, zap.String("tenant_id", "tenant1"))
	ctx = AddFields(ctx, zap.String("request_id", "req123"))

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("Expected non-nil logger after chaining")
	}

	// Should not panic
	retrieved.Info("chained context test")
}

func TestMultipleLoggers(t *testing.T) {
	logger1, _ := NewDevelopmentLogger()
	logger2, _ := NewProductionLogger("info")

	ctx := context.Background()

	// Store first logger
	ctx = WithLogger(ctx, logger1)
	retrieved1 := FromContext(ctx)

	// Replace with second logger
	ctx = WithLogger(ctx, logger2)
	retrieved2 := FromContext(ctx)

	if retrieved1 == nil || retrieved2 == nil {
		t.Fatal("Expected non-nil loggers")
	}

	// Both should work without panicking
	retrieved1.Info("logger 1")
	retrieved2.Info("logger 2")
}
