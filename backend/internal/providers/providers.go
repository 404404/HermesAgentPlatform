package providers

import (
	"context"
	"fmt"
	"sync"
)

// RuntimeProvider decouples the control plane from the process/container that
// will eventually host a user's Hermes runtime.
type RuntimeProvider interface {
	CreateRuntime(ctx context.Context, userID int64) (string, error)
	StartRuntime(ctx context.Context, runtimeID string) error
	StopRuntime(ctx context.Context, runtimeID string) error
	RestartRuntime(ctx context.Context, runtimeID string) error
	DeleteRuntime(ctx context.Context, runtimeID string) error
	GetRuntimeStatus(ctx context.Context, runtimeID string) (string, error)
}

// MockRuntimeProvider intentionally keeps state in memory. It makes the MVP
// demonstrable without granting the backend access to the Docker socket.
type MockRuntimeProvider struct {
	mu     sync.RWMutex
	status map[string]string
}

func NewMockRuntimeProvider() *MockRuntimeProvider {
	return &MockRuntimeProvider{status: map[string]string{}}
}

func (p *MockRuntimeProvider) CreateRuntime(_ context.Context, userID int64) (string, error) {
	id := fmt.Sprintf("mock-runtime-%d", userID)
	p.mu.Lock()
	p.status[id] = "stopped"
	p.mu.Unlock()
	return id, nil
}

func (p *MockRuntimeProvider) StartRuntime(_ context.Context, runtimeID string) error {
	return p.set(runtimeID, "running")
}

func (p *MockRuntimeProvider) StopRuntime(_ context.Context, runtimeID string) error {
	return p.set(runtimeID, "stopped")
}

func (p *MockRuntimeProvider) RestartRuntime(_ context.Context, runtimeID string) error {
	return p.set(runtimeID, "running")
}

func (p *MockRuntimeProvider) DeleteRuntime(_ context.Context, runtimeID string) error {
	p.mu.Lock()
	delete(p.status, runtimeID)
	p.mu.Unlock()
	return nil
}

func (p *MockRuntimeProvider) GetRuntimeStatus(_ context.Context, runtimeID string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	status, ok := p.status[runtimeID]
	if !ok {
		return "unknown", nil
	}
	return status, nil
}

func (p *MockRuntimeProvider) set(runtimeID, status string) error {
	p.mu.Lock()
	p.status[runtimeID] = status
	p.mu.Unlock()
	return nil
}

// KnowledgeProvider is the future seam for a vector/search backend.
type KnowledgeProvider interface {
	Search(ctx context.Context, knowledgeBaseID int64, query string) ([]string, error)
	IndexDocument(ctx context.Context, knowledgeBaseID int64, name string, content []byte) error
	DeleteDocument(ctx context.Context, knowledgeBaseID int64, documentID string) error
	GetDocument(ctx context.Context, knowledgeBaseID int64, documentID string) ([]byte, error)
}

type MockKnowledgeProvider struct{}

func (MockKnowledgeProvider) Search(_ context.Context, _ int64, _ string) ([]string, error) {
	return []string{}, nil
}
func (MockKnowledgeProvider) IndexDocument(_ context.Context, _ int64, _ string, _ []byte) error {
	return nil
}
func (MockKnowledgeProvider) DeleteDocument(_ context.Context, _ int64, _ string) error {
	return nil
}
func (MockKnowledgeProvider) GetDocument(_ context.Context, _ int64, _ string) ([]byte, error) {
	return nil, nil
}

// UsageCollector keeps usage ingestion independent from Hermes telemetry.
type UsageCollector interface {
	Collect(ctx context.Context, event UsageEvent) error
}

type UsageEvent struct {
	OrganizationID int64
	DepartmentID   int64
	UserID         int64
	ProfileID      int64
	SessionID      string
	ExecutionID    string
	ModelID        int64
	SkillID        int64
	RuntimeID      int64
	TokenInput     int64
	TokenOutput    int64
	Requests       int64
	Executions     int64
	LatencyMS      int64
}

type MockUsageCollector struct{}

func (MockUsageCollector) Collect(_ context.Context, _ UsageEvent) error { return nil }
