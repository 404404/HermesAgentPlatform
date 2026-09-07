package providers

import (
	"context"
	"time"
)

// SecretInput is an internal-only operational credential payload. It must
// never be populated from, or returned to, ordinary list/detail APIs.
type SecretInput struct {
	OrganizationID int64
	Name           string
	Type           string
	Scope          string
	OwnerUserID    *int64
	Value          []byte
}

// SecretMetadata is intentionally value-free. It is safe for management APIs
// after authorization has been checked by their caller.
type SecretMetadata struct {
	ID              int64
	Name            string
	Type            string
	Scope           string
	Status          string
	RequiresReentry bool
	LastUpdated     *time.Time
}

// SecretProvider keeps secret value handling behind a narrow backend-only
// interface. DecryptSecretForAuthorizedUse is for integrations such as a
// runtime host or model provider; it must never be wired to a browser API.
type SecretProvider interface {
	CreateSecret(ctx context.Context, input SecretInput) (SecretMetadata, error)
	UpdateSecret(ctx context.Context, id int64, value []byte) (SecretMetadata, error)
	DecryptSecretForAuthorizedUse(ctx context.Context, id int64) ([]byte, error)
	GetSecretMetadata(ctx context.Context, id int64) (SecretMetadata, error)
	DeleteSecret(ctx context.Context, id int64) error
}
