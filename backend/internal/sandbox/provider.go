package sandbox

import "context"

// Provider is deliberately a narrow seam. No privileged Docker or host
// mounts are needed by the MVP.
type Provider interface {
	CreateSandbox(ctx context.Context, profileID int64, class string) (string, error)
	StartSandbox(ctx context.Context, sandboxID string) error
	Exec(ctx context.Context, sandboxID, command string) (string, error)
	StopSandbox(ctx context.Context, sandboxID string) error
	DestroySandbox(ctx context.Context, sandboxID string) error
	GetStatus(ctx context.Context, sandboxID string) (string, error)
}

type MockProvider struct{}

func (MockProvider) CreateSandbox(_ context.Context, profileID int64, _ string) (string, error) {
	return "sandbox-not-provisioned-" + string(rune(profileID)), nil
}
func (MockProvider) StartSandbox(_ context.Context, _ string) error      { return nil }
func (MockProvider) Exec(_ context.Context, _, _ string) (string, error) { return "", nil }
func (MockProvider) StopSandbox(_ context.Context, _ string) error       { return nil }
func (MockProvider) DestroySandbox(_ context.Context, _ string) error    { return nil }
func (MockProvider) GetStatus(_ context.Context, _ string) (string, error) {
	return "not_provisioned", nil
}
