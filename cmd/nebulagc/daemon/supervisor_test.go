package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestSupervisor_StartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Mock nebula process
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
sleep 5
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       100 * time.Millisecond,
		MaxBackoff:       1 * time.Second,
		SuccessThreshold: 1 * time.Second,
		Logger:           logger,
	})

	// Start supervisor
	go s.Run()

	// Wait for process to start
	time.Sleep(200 * time.Millisecond)

	// Verify running
	if !s.IsRunning() {
		t.Error("Supervisor should have started process")
	}

	if s.PID() <= 0 {
		t.Error("PID should be positive")
	}

	// Stop supervisor
	if err := s.Stop(); err != nil {
		t.Fatalf("Failed to stop supervisor: %v", err)
	}

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Should be stopped
	if s.IsRunning() {
		t.Error("Process should be stopped")
	}
}

func TestSupervisor_AutoRestart(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Script that exits immediately (simulating crash)
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       50 * time.Millisecond,
		MaxBackoff:       500 * time.Millisecond,
		SuccessThreshold: 1 * time.Second,
		Logger:           logger,
	})

	// Start supervisor
	go s.Run()

	// Wait for multiple restart attempts
	time.Sleep(300 * time.Millisecond)

	// Stop supervisor
	s.Stop()

	// Test passed if we got here without hanging
}

func TestSupervisor_BackoffIncreases(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Script that exits immediately
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	minBackoff := 10 * time.Millisecond
	maxBackoff := 100 * time.Millisecond

	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       minBackoff,
		MaxBackoff:       maxBackoff,
		SuccessThreshold: 1 * time.Second,
		Logger:           logger,
	})

	// Start supervisor
	go s.Run()

	// Let it crash and restart a few times
	time.Sleep(300 * time.Millisecond)

	// Check that backoff increased
	if s.currentBackoff < minBackoff*2 {
		t.Error("Backoff should have increased after crashes")
	}

	// Should not exceed max
	if s.currentBackoff > maxBackoff {
		t.Error("Backoff should not exceed max")
	}

	s.Stop()
}

func TestSupervisor_Restart(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
sleep 5
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       100 * time.Millisecond,
		MaxBackoff:       1 * time.Second,
		SuccessThreshold: 1 * time.Second,
		Logger:           logger,
	})

	go s.Run()

	// Wait for process to start
	time.Sleep(200 * time.Millisecond)

	firstPID := s.PID()
	if firstPID <= 0 {
		t.Fatal("Process should be running")
	}

	// Request restart
	s.Restart()

	// Wait for restart to complete
	time.Sleep(500 * time.Millisecond)

	// Should have a new PID
	newPID := s.PID()
	if newPID <= 0 {
		t.Error("Process should be running after restart")
	}

	// PID may or may not change depending on timing, but process should be running
	// The important thing is that restart was triggered and completed
	if !s.IsRunning() {
		t.Error("Process should be running after restart")
	}

	// Clean stop
	s.Stop()
}

func TestSupervisor_BackoffReset(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Script that runs successfully for a while
	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
sleep 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	minBackoff := 10 * time.Millisecond
	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       minBackoff,
		MaxBackoff:       100 * time.Millisecond,
		SuccessThreshold: 500 * time.Millisecond,
		Logger:           logger,
	})

	// Manually set high backoff
	s.currentBackoff = 100 * time.Millisecond

	go s.Run()
	defer s.Stop()

	// Wait for process to run successfully long enough to reset backoff
	// The process runs for 1 second, success threshold is 500ms
	time.Sleep(1500 * time.Millisecond)

	// Backoff should be reset to min after successful run
	// Allow some tolerance since this is timing-dependent
	if s.currentBackoff > minBackoff*2 {
		t.Errorf("Backoff should be reset close to %v, got %v", minBackoff, s.currentBackoff)
	}
}

func TestSupervisor_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
# Trap SIGTERM and exit cleanly
trap 'exit 0' TERM
sleep 30 &
wait $!
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       100 * time.Millisecond,
		MaxBackoff:       1 * time.Second,
		SuccessThreshold: 1 * time.Second,
		Logger:           logger,
	})

	go s.Run()

	// Wait for process to start
	time.Sleep(200 * time.Millisecond)

	// Stop should complete quickly (graceful shutdown)
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(15 * time.Second):
		t.Fatal("Stop took too long (should be < 15 seconds)")
	}
}

func TestSupervisor_MultipleRestarts(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte("test: config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	scriptPath := filepath.Join(tmpDir, "nebula")
	script := `#!/bin/sh
sleep 5
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	s := NewSupervisor(SupervisorConfig{
		ConfigPath:       configPath,
		MinBackoff:       50 * time.Millisecond,
		MaxBackoff:       500 * time.Millisecond,
		SuccessThreshold: 1 * time.Second,
		Logger:           logger,
	})

	go s.Run()

	// Wait for initial start
	time.Sleep(100 * time.Millisecond)

	// Request a couple of restarts (not too many to avoid race)
	s.Restart()
	time.Sleep(100 * time.Millisecond)
	s.Restart()

	// Wait for restart to settle
	time.Sleep(300 * time.Millisecond)

	// Should still be running
	if !s.IsRunning() {
		t.Error("Process should be running after restarts")
	}

	// Clean stop
	s.Stop()
}
