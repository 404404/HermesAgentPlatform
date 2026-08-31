package hermes

import "context"

// Adapter owns Hermes-specific translation. HTTP handlers and repositories do
// not depend on ~/.hermes or any upstream file layout.
type Adapter interface {
	BuildProfileConfig(ctx context.Context, profile ProfileSpec) (map[string]any, error)
	BuildRuntimeConfig(ctx context.Context, runtime RuntimeSpec) (map[string]any, error)
	Version(ctx context.Context) (string, error)
	ParseUsage(ctx context.Context, payload []byte) ([]UsageRecord, error)
}

type ProfileSpec struct {
	Name        string
	DisplayName string
	Model       string
}

type RuntimeSpec struct {
	RuntimeID string
	UserID    int64
}

type UsageRecord struct {
	InputTokens  int64
	OutputTokens int64
}

type MockAdapter struct{}

func (MockAdapter) BuildProfileConfig(_ context.Context, p ProfileSpec) (map[string]any, error) {
	return map[string]any{"profile_name": p.Name, "display_name": p.DisplayName, "model": p.Model}, nil
}
func (MockAdapter) BuildRuntimeConfig(_ context.Context, r RuntimeSpec) (map[string]any, error) {
	return map[string]any{"runtime_id": r.RuntimeID, "user_id": r.UserID}, nil
}
func (MockAdapter) Version(_ context.Context) (string, error) { return "mock-hermes-0.1", nil }
func (MockAdapter) ParseUsage(_ context.Context, _ []byte) ([]UsageRecord, error) {
	return []UsageRecord{}, nil
}
