package logging

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger_Development(t *testing.T) {
	cfg := Config{
		Level:            "debug",
		Environment:      EnvironmentDevelopment,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create development logger: %v", err)
	}

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	// Should not panic
	logger.Debug("test debug message")
	logger.Info("test info message")
}

func TestNewLogger_Production(t *testing.T) {
	cfg := Config{
		Level:            "info",
		Environment:      EnvironmentProduction,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create production logger: %v", err)
	}

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	// Should not panic
	logger.Info("test info message")
	logger.Error("test error message")
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	cfg := Config{
		Level:            "invalid",
		Environment:      EnvironmentDevelopment,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	_, err := NewLogger(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid log level")
	}
}

func TestNewLogger_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := Config{
				Level:            level,
				Environment:      EnvironmentProduction,
				OutputPaths:      []string{"stdout"},
				ErrorOutputPaths: []string{"stderr"},
			}

			logger, err := NewLogger(cfg)
			if err != nil {
				t.Fatalf("Failed to create logger with level %s: %v", level, err)
			}

			if logger == nil {
				t.Fatalf("Expected non-nil logger for level %s", level)
			}
		})
	}
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	if err != nil {
		t.Fatalf("Failed to create development logger: %v", err)
	}

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	// Should not panic
	logger.Debug("test message")
}

func TestNewProductionLogger(t *testing.T) {
	logger, err := NewProductionLogger("info")
	if err != nil {
		t.Fatalf("Failed to create production logger: %v", err)
	}

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	// Should not panic
	logger.Info("test message")
}

func TestNewProductionLogger_EmptyLevel(t *testing.T) {
	logger, err := NewProductionLogger("")
	if err != nil {
		t.Fatalf("Failed to create production logger with empty level: %v", err)
	}

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("Expected default level 'info', got %s", cfg.Level)
	}

	if cfg.Environment != EnvironmentDevelopment {
		t.Errorf("Expected development environment, got %s", cfg.Environment)
	}

	if len(cfg.OutputPaths) != 1 || cfg.OutputPaths[0] != "stdout" {
		t.Errorf("Expected output paths [stdout], got %v", cfg.OutputPaths)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
		hasError bool
	}{
		{"debug", zapcore.DebugLevel, false},
		{"info", zapcore.InfoLevel, false},
		{"warn", zapcore.WarnLevel, false},
		{"error", zapcore.ErrorLevel, false},
		{"DEBUG", zapcore.DebugLevel, false}, // Should handle uppercase
		{"invalid", zapcore.DebugLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %s", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %s: %v", tt.input, err)
				}
				if level != tt.expected {
					t.Errorf("Expected level %v, got %v", tt.expected, level)
				}
			}
		})
	}
}

func TestMustNewLogger(t *testing.T) {
	// Should not panic with valid config
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustNewLogger panicked with valid config: %v", r)
		}
	}()

	cfg := DefaultConfig()
	logger := MustNewLogger(cfg)

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
}

func TestMustNewLogger_Panic(t *testing.T) {
	// Should panic with invalid config
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected MustNewLogger to panic with invalid config")
		}
	}()

	cfg := Config{
		Level:       "invalid",
		Environment: EnvironmentProduction,
	}

	MustNewLogger(cfg)
}

func TestEncodingFromEnvironment(t *testing.T) {
	tests := []struct {
		env      Environment
		expected string
	}{
		{EnvironmentProduction, "json"},
		{EnvironmentDevelopment, "console"},
	}

	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			result := encodingFromEnvironment(tt.env)
			if result != tt.expected {
				t.Errorf("Expected encoding %s for %s, got %s", tt.expected, tt.env, result)
			}
		})
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger, err := NewDevelopmentLogger()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Should not panic when adding fields
	logger.With(
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
	).Info("test message with fields")
}
