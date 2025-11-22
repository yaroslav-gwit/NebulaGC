package ha

import (
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type mockRegistry struct {
	mu sync.Mutex

	registerCalls   int
	unregisterCalls int
	validateCalls   int
	heartbeatCalls  int
	pruneCalls      int

	registerArgs struct {
		id, addr string
		mode     Mode
	}
	masterInfo *MasterInfo
	list       []*ReplicaInfo

	registerErr  error
	validateErr  error
	heartbeatErr error
	pruneErr     error
}

func (m *mockRegistry) Register(instanceID, address string, mode Mode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerCalls++
	m.registerArgs = struct {
		id   string
		addr string
		mode Mode
	}{instanceID, address, mode}
	return m.registerErr
}

func (m *mockRegistry) ValidateSingleMaster() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateCalls++
	return m.validateErr
}

func (m *mockRegistry) SendHeartbeat(string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeatCalls++
	return m.heartbeatErr
}

func (m *mockRegistry) PruneStale(time.Duration, int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneCalls++
	return 0, m.pruneErr
}

func (m *mockRegistry) GetMaster(time.Duration, string) (*MasterInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.masterInfo != nil {
		return m.masterInfo, nil
	}
	return &MasterInfo{InstanceID: "self", Address: "addr", IsSelf: true}, nil
}

func (m *mockRegistry) ListReplicas(time.Duration, string) ([]*ReplicaInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.list, nil
}

func (m *mockRegistry) Unregister(string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unregisterCalls++
	return nil
}

func newTestHAManager(cfg *Config, reg *mockRegistry) *Manager {
	core, _ := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	return NewManager(cfg, reg, logger)
}

func TestManagerStartValidatesMaster(t *testing.T) {
	reg := &mockRegistry{}
	cfg := &Config{
		InstanceID:         "self",
		Address:            "https://self.example.com",
		Mode:               ModeMaster,
		HeartbeatInterval:  5 * time.Millisecond,
		HeartbeatThreshold: 10 * time.Millisecond,
		PruneInterval:      5 * time.Millisecond,
		EnablePruning:      true,
	}

	manager := newTestHAManager(cfg, reg)
	if err := manager.Start(); err != nil {
		t.Fatalf("manager start failed: %v", err)
	}
	time.Sleep(15 * time.Millisecond)
	if err := manager.Stop(); err != nil {
		t.Fatalf("manager stop failed: %v", err)
	}

	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.registerCalls == 0 || reg.unregisterCalls == 0 {
		t.Fatalf("expected register and unregister calls, got register=%d unregister=%d", reg.registerCalls, reg.unregisterCalls)
	}
	if reg.validateCalls == 0 {
		t.Fatal("expected master validation to be called")
	}
	if reg.heartbeatCalls == 0 {
		t.Fatal("expected heartbeat loop to run")
	}
	if reg.pruneCalls == 0 {
		t.Fatal("expected prune loop to run")
	}
}

func TestManagerStartReplicaSkipsValidation(t *testing.T) {
	reg := &mockRegistry{}
	cfg := &Config{
		InstanceID:         "self",
		Address:            "https://self.example.com",
		Mode:               ModeReplica,
		HeartbeatInterval:  0,
		HeartbeatThreshold: 0,
		PruneInterval:      0,
		EnablePruning:      false,
	}

	manager := newTestHAManager(cfg, reg)
	if err := manager.Start(); err != nil {
		t.Fatalf("manager start failed: %v", err)
	}
	if err := manager.Stop(); err != nil {
		t.Fatalf("manager stop failed: %v", err)
	}

	reg.mu.Lock()
	defer reg.mu.Unlock()
	if reg.validateCalls != 0 {
		t.Fatalf("expected no master validation on replica, got %d", reg.validateCalls)
	}
	if cfg.HeartbeatInterval == 0 || cfg.HeartbeatThreshold == 0 || cfg.PruneInterval == 0 {
		t.Fatal("expected defaults to be applied")
	}
}

func TestManagerStartValidationError(t *testing.T) {
	reg := &mockRegistry{validateErr: errors.New("too many masters")}
	cfg := &Config{
		InstanceID:         "self",
		Address:            "https://self.example.com",
		Mode:               ModeMaster,
		HeartbeatInterval:  5 * time.Millisecond,
		HeartbeatThreshold: 10 * time.Millisecond,
		PruneInterval:      5 * time.Millisecond,
		EnablePruning:      true,
	}

	manager := newTestHAManager(cfg, reg)
	if err := manager.Start(); err == nil {
		t.Fatal("expected start to fail when validation fails")
	}
}
