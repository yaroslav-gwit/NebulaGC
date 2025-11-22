// Package daemon provides the daemon process management functionality.
package daemon

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Process wraps a Nebula process with monitoring and log capture.
type Process struct {
	cmd        *exec.Cmd
	configPath string
	logger     *zap.Logger

	mu      sync.RWMutex
	running bool
	pid     int
}

// NewProcess creates a new Nebula process wrapper.
func NewProcess(configPath string, logger *zap.Logger) *Process {
	return &Process{
		configPath: configPath,
		logger:     logger,
	}
}

// Start starts the Nebula process.
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	// Create command
	p.cmd = exec.CommandContext(ctx, "nebula", "-config", p.configPath)

	// Setup stdout/stderr capture
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Start the process
	if err := p.cmd.Start(); err != nil {
		return err
	}

	p.running = true
	p.pid = p.cmd.Process.Pid

	p.logger.Info("nebula process started",
		zap.Int("pid", p.pid),
		zap.String("config", p.configPath))

	// Capture logs in background
	go p.captureOutput(stdout, "stdout")
	go p.captureOutput(stderr, "stderr")

	return nil
}

// Wait waits for the process to exit and returns the exit code.
func (p *Process) Wait() error {
	p.mu.RLock()
	cmd := p.cmd
	p.mu.RUnlock()

	if cmd == nil {
		return nil
	}

	err := cmd.Wait()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	if err != nil {
		// Get PID for logging
		p.mu.RLock()
		pid := p.pid
		p.mu.RUnlock()

		// Check if it was killed by signal
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			if status.Signaled() {
				p.logger.Info("nebula process killed by signal",
					zap.String("signal", status.Signal().String()),
					zap.Int("pid", pid))
				return nil
			}
			p.logger.Error("nebula process exited with error",
				zap.Int("exit_code", exitErr.ExitCode()),
				zap.Int("pid", pid))
		}
		return err
	}

	// Get PID for logging
	p.mu.RLock()
	pid := p.pid
	p.mu.RUnlock()

	p.logger.Info("nebula process exited normally",
		zap.Int("pid", pid))

	return nil
}

// Stop stops the Nebula process gracefully.
func (p *Process) Stop() error {
	p.mu.Lock()

	if !p.running || p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		return nil
	}

	// Get references under lock
	proc := p.cmd.Process
	pid := p.pid

	p.mu.Unlock()

	p.logger.Info("stopping nebula process",
		zap.Int("pid", pid))

	// Send SIGTERM for graceful shutdown
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	// Wait for process to exit (up to 10 seconds)
	done := make(chan struct{})
	var waitErr error

	go func() {
		// Call Wait without holding any locks
		p.mu.RLock()
		cmd := p.cmd
		p.mu.RUnlock()

		if cmd != nil {
			waitErr = cmd.Wait()
		}
		close(done)
	}()

	select {
	case <-done:
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()

		p.logger.Info("nebula process stopped gracefully",
			zap.Int("pid", pid))
		return waitErr
	case <-time.After(10 * time.Second):
		// Force kill if not responding
		p.logger.Warn("nebula process not responding to SIGTERM, force killing",
			zap.Int("pid", pid))

		if err := proc.Kill(); err != nil {
			return err
		}

		p.mu.Lock()
		p.running = false
		p.mu.Unlock()

		return nil
	}
}

// IsRunning returns whether the process is currently running.
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// PID returns the process ID.
func (p *Process) PID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

// captureOutput captures and logs process output.
func (p *Process) captureOutput(reader io.Reader, source string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Get PID under read lock
		p.mu.RLock()
		pid := p.pid
		p.mu.RUnlock()

		p.logger.Info("nebula output",
			zap.String("source", source),
			zap.String("line", line),
			zap.Int("pid", pid))
	}

	if err := scanner.Err(); err != nil {
		// Get PID under read lock
		p.mu.RLock()
		pid := p.pid
		p.mu.RUnlock()

		p.logger.Error("error reading nebula output",
			zap.Error(err),
			zap.String("source", source),
			zap.Int("pid", pid))
	}
}
