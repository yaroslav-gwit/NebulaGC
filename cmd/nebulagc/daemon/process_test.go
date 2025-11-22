package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestProcess_StartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a test config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Use 'sleep' command instead of nebula for testing
	// We'll mock the nebula command by creating a wrapper script
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
# Mock nebula process - just sleep
sleep 30
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Temporarily add script dir to PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	// Create process
	p := NewProcess(configPath, logger)

	// Start process
	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Verify running
	if !p.IsRunning() {
		t.Error("Process should be running")
	}

	if p.PID() <= 0 {
		t.Error("PID should be positive")
	}

	// Stop process
	if err := p.Stop(); err != nil {
		t.Fatalf("Failed to stop process: %v", err)
	}

	// Wait a bit for stop to complete
	time.Sleep(100 * time.Millisecond)

	// Verify stopped
	if p.IsRunning() {
		t.Error("Process should be stopped")
	}
}

func TestProcess_Wait(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create a script that exits quickly
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
sleep 0.1
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := NewProcess(configPath, logger)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for process to exit
	err := p.Wait()
	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}

	// Should be stopped now
	if p.IsRunning() {
		t.Error("Process should be stopped after Wait")
	}
}

func TestProcess_StartTwice(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
sleep 30
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := NewProcess(configPath, logger)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer p.Stop()

	// Try to start again - should be no-op
	if err := p.Start(ctx); err != nil {
		t.Errorf("Second Start should not error: %v", err)
	}

	// Should still be running
	if !p.IsRunning() {
		t.Error("Process should still be running")
	}
}

func TestProcess_OutputCapture(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Script that produces output
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
echo "stdout message"
echo "stderr message" >&2
sleep 0.1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	p := NewProcess(configPath, logger)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for process to complete
	p.Wait()

	// Output should have been logged (we can't easily verify this without
	// a custom logger implementation, but at least it shouldn't crash)
}

func TestProcess_StopNotRunning(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	p := NewProcess(configPath, logger)

	// Try to stop without starting - should not error
	if err := p.Stop(); err != nil {
		t.Errorf("Stop on non-running process should not error: %v", err)
	}
}

func TestProcess_MissingNebulaBinary(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Set PATH to empty dir so nebula won't be found
	emptyDir := filepath.Join(tmpDir, "empty")
	os.Mkdir(emptyDir, 0755)
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", emptyDir)
	defer os.Setenv("PATH", oldPath)

	p := NewProcess(configPath, logger)

	ctx := context.Background()
	err := p.Start(ctx)
	if err == nil {
		t.Error("Start should fail when nebula binary is missing")
		p.Stop()
	}

	// Should be a exec.ErrNotFound
	if _, ok := err.(*exec.Error); !ok {
		t.Errorf("Expected exec.Error, got: %v", err)
	}
}
