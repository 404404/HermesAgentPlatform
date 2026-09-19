package providers

import "context"

// AuthProvider authenticates external identities without making them RBAC
// principals. Local Account is the Phase 1 implementation seam.
type AuthProvider interface {
	Authenticate(ctx context.Context, credential map[string]string) (ExternalIdentity, error)
}

type ExternalIdentity struct {
	ProviderType    string
	ProviderID      string
	ExternalSubject string
}

type LocalAuthProvider struct{}

func (LocalAuthProvider) Authenticate(_ context.Context, _ map[string]string) (ExternalIdentity, error) {
	return ExternalIdentity{ProviderType: "local", ProviderID: "local"}, nil
}
