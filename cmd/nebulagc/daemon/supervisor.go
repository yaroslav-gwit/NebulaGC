// Package daemon provides the daemon process management functionality.
package daemon

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Supervisor manages the lifecycle of a Nebula process with automatic restart.
type Supervisor struct {
	mu         sync.RWMutex // Protects process, currentBackoff fields
	process    *Process
	configPath string
	logger     *zap.Logger

	// Restart backoff settings
	minBackoff       time.Duration
	maxBackoff       time.Duration
	currentBackoff   time.Duration // Protected by mu
	successThreshold time.Duration // Reset backoff if process runs this long

	// Control
	ctx        context.Context
	cancelFunc context.CancelFunc
	stopCh     chan struct{}
	restartCh  chan struct{} // Signal to restart process
}

// SupervisorConfig holds configuration for the supervisor.
type SupervisorConfig struct {
	ConfigPath       string
	MinBackoff       time.Duration
	MaxBackoff       time.Duration
	SuccessThreshold time.Duration
	Logger           *zap.Logger
}

// NewSupervisor creates a new process supervisor.
func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())

	// Set defaults
	if cfg.MinBackoff == 0 {
		cfg.MinBackoff = 1 * time.Second
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = 60 * time.Second
	}
	if cfg.SuccessThreshold == 0 {
		cfg.SuccessThreshold = 5 * time.Minute
	}

	return &Supervisor{
		configPath:       cfg.ConfigPath,
		logger:           cfg.Logger,
		minBackoff:       cfg.MinBackoff,
		maxBackoff:       cfg.MaxBackoff,
		currentBackoff:   cfg.MinBackoff,
		successThreshold: cfg.SuccessThreshold,
		ctx:              ctx,
		cancelFunc:       cancel,
		stopCh:           make(chan struct{}),
		restartCh:        make(chan struct{}, 1),
	}
}

// Run starts the supervisor loop.
func (s *Supervisor) Run() error {
	s.logger.Info("supervisor starting",
		zap.String("config", s.configPath))

	defer close(s.stopCh)

	for {
		// Check for shutdown before starting
		if s.ctx.Err() != nil {
			s.logger.Info("supervisor stopping")
			return nil
		}

		// Start process
		if err := s.startProcess(); err != nil {
			s.logger.Error("failed to start process", zap.Error(err))
			s.applyBackoff()
			continue
		}

		// Wait for process to exit or restart signal
		startTime := time.Now()

		// Wait in goroutine so we can handle restart signals
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- s.process.Wait()
		}()

		// Wait for exit or signals
		var shouldRestart bool
		select {
		case <-s.ctx.Done():
			s.logger.Info("supervisor stopping")

			s.mu.RLock()
			proc := s.process
			s.mu.RUnlock()

			if proc != nil && proc.IsRunning() {
				if err := proc.Stop(); err != nil {
					s.logger.Error("error stopping process", zap.Error(err))
				}
			}
			// Drain the wait channel with timeout
			select {
			case <-waitCh:
			case <-time.After(2 * time.Second):
				s.logger.Warn("timeout waiting for process to exit")
			}
			return nil

		case <-s.restartCh:
			s.logger.Info("restart requested")

			s.mu.RLock()
			proc := s.process
			s.mu.RUnlock()

			if proc != nil && proc.IsRunning() {
				if err := proc.Stop(); err != nil {
					s.logger.Error("error stopping process for restart", zap.Error(err))
				}
			}
			// Wait for process to actually exit
			<-waitCh
			// Restart immediately without backoff
			shouldRestart = true

		case err := <-waitCh:
			// Process exited naturally
			runDuration := time.Since(startTime)

			if err != nil {
				s.logger.Error("process exited with error",
					zap.Error(err),
					zap.Duration("run_duration", runDuration))
				s.applyBackoff()
			} else {
				s.logger.Info("process exited normally",
					zap.Duration("run_duration", runDuration))
			}

			// Reset backoff if process ran long enough
			if runDuration >= s.successThreshold {
				s.logger.Info("process ran successfully, resetting backoff",
					zap.Duration("run_duration", runDuration))
				s.mu.Lock()
				s.currentBackoff = s.minBackoff
				s.mu.Unlock()
			}
			shouldRestart = true
		}

		if shouldRestart {
			// Continue to next iteration to restart
			continue
		}
	}
} // Stop stops the supervisor and the managed process.
func (s *Supervisor) Stop() error {
	s.logger.Info("stopping supervisor")
	s.cancelFunc()
	<-s.stopCh // Wait for supervisor to finish
	return nil
}

// Restart signals the supervisor to restart the process.
func (s *Supervisor) Restart() {
	select {
	case s.restartCh <- struct{}{}:
		s.logger.Info("restart signal sent")
	default:
		// Already have a restart pending
		s.logger.Debug("restart already pending")
	}
}

// startProcess starts a new Nebula process.
func (s *Supervisor) startProcess() error {
	proc := NewProcess(s.configPath, s.logger)
	if err := proc.Start(s.ctx); err != nil {
		return err
	}

	s.mu.Lock()
	s.process = proc
	s.mu.Unlock()

	return nil
}

// applyBackoff applies exponential backoff before restarting.
func (s *Supervisor) applyBackoff() {
	s.mu.RLock()
	backoff := s.currentBackoff
	s.mu.RUnlock()

	s.logger.Info("applying restart backoff",
		zap.Duration("delay", backoff))

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Backoff completed
	case <-s.ctx.Done():
		// Supervisor stopping
		return
	}

	// Increase backoff for next time (exponential)
	s.mu.Lock()
	s.currentBackoff *= 2
	if s.currentBackoff > s.maxBackoff {
		s.currentBackoff = s.maxBackoff
	}
	s.mu.Unlock()
}

// IsRunning returns whether the supervised process is running.
func (s *Supervisor) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.process == nil {
		return false
	}
	return s.process.IsRunning()
}

// PID returns the process ID of the supervised process.
func (s *Supervisor) PID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.process == nil {
		return 0
	}
	return s.process.PID()
}
