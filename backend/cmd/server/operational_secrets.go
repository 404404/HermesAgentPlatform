package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/example/hermes-enterprise-platform/backend/internal/providers"
)

const (
	operationalSecretAlgorithm  = "AES-256-GCM"
	operationalSecretKeyVersion = "v1"
)

var (
	errSecretRequiresReentry = errors.New("secret requires re-entry")
	errSecretNotConfigured   = errors.New("secret is not configured")
)

// databaseSecretProvider encrypts operational credentials at rest. It is
// deliberately separate from users.password_hash, which stays bcrypt-only.
type databaseSecretProvider struct {
	db  *sql.DB
	key []byte
}

var _ providers.SecretProvider = (*databaseSecretProvider)(nil)

func parseOperationalSecretMasterKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("HEP_SECRET_MASTER_KEY must be set")
	}
	if strings.HasPrefix(trimmed, "base64:") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(trimmed, "base64:"))
		if err != nil {
			return nil, fmt.Errorf("HEP_SECRET_MASTER_KEY base64 decode: %w", err)
		}
		trimmed = string(decoded)
	}
	if len(trimmed) != 32 {
		return nil, errors.New("HEP_SECRET_MASTER_KEY must be exactly 32 bytes (or base64: encoded 32 bytes)")
	}
	return []byte(trimmed), nil
}

func newDatabaseSecretProvider(db *sql.DB, masterKey string) (*databaseSecretProvider, error) {
	key, err := parseOperationalSecretMasterKey(masterKey)
	if err != nil {
		return nil, err
	}
	return &databaseSecretProvider{db: db, key: key}, nil
}

func (p *databaseSecretProvider) encrypt(value []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, value, nil), nonce, nil
}

func (p *databaseSecretProvider) decrypt(ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (p *databaseSecretProvider) metadata(ctx context.Context, id int64) (providers.SecretMetadata, error) {
	var metadata providers.SecretMetadata
	var updated sql.NullTime
	err := p.db.QueryRowContext(ctx, `SELECT id,name,type,scope,status,requires_reentry,last_updated FROM secrets WHERE id=?`, id).Scan(
		&metadata.ID, &metadata.Name, &metadata.Type, &metadata.Scope, &metadata.Status, &metadata.RequiresReentry, &updated,
	)
	if err != nil {
		return providers.SecretMetadata{}, err
	}
	if updated.Valid {
		value := updated.Time
		metadata.LastUpdated = &value
	}
	return metadata, nil
}

func (p *databaseSecretProvider) CreateSecret(ctx context.Context, input providers.SecretInput) (providers.SecretMetadata, error) {
	if len(input.Value) == 0 {
		return providers.SecretMetadata{}, errSecretNotConfigured
	}
	ciphertext, nonce, err := p.encrypt(input.Value)
	if err != nil {
		return providers.SecretMetadata{}, err
	}
	_, err = p.db.ExecContext(ctx, `INSERT INTO secrets(organization_id,name,type,scope,owner_user_id,status,encrypted_value,ciphertext,nonce,algorithm,key_version,requires_reentry,last_updated)
		VALUES(?,?,?,?,?,'configured',NULL,?,?,?, ?,FALSE,UTC_TIMESTAMP())
		ON DUPLICATE KEY UPDATE type=VALUES(type),scope=VALUES(scope),owner_user_id=VALUES(owner_user_id),status='configured',encrypted_value=NULL,ciphertext=VALUES(ciphertext),nonce=VALUES(nonce),algorithm=VALUES(algorithm),key_version=VALUES(key_version),requires_reentry=FALSE,last_updated=UTC_TIMESTAMP()`,
		input.OrganizationID, input.Name, input.Type, input.Scope, input.OwnerUserID, ciphertext, nonce, operationalSecretAlgorithm, operationalSecretKeyVersion)
	if err != nil {
		return providers.SecretMetadata{}, err
	}
	var id int64
	if err := p.db.QueryRowContext(ctx, `SELECT id FROM secrets WHERE organization_id=? AND name=?`, input.OrganizationID, input.Name).Scan(&id); err != nil {
		return providers.SecretMetadata{}, err
	}
	return p.metadata(ctx, id)
}

func (p *databaseSecretProvider) UpdateSecret(ctx context.Context, id int64, value []byte) (providers.SecretMetadata, error) {
	if len(value) == 0 {
		return p.metadata(ctx, id)
	}
	ciphertext, nonce, err := p.encrypt(value)
	if err != nil {
		return providers.SecretMetadata{}, err
	}
	result, err := p.db.ExecContext(ctx, `UPDATE secrets SET status='configured',encrypted_value=NULL,ciphertext=?,nonce=?,algorithm=?,key_version=?,requires_reentry=FALSE,last_updated=UTC_TIMESTAMP() WHERE id=?`, ciphertext, nonce, operationalSecretAlgorithm, operationalSecretKeyVersion, id)
	if err != nil {
		return providers.SecretMetadata{}, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return providers.SecretMetadata{}, sql.ErrNoRows
	}
	return p.metadata(ctx, id)
}

func (p *databaseSecretProvider) DecryptSecretForAuthorizedUse(ctx context.Context, id int64) ([]byte, error) {
	var ciphertext, nonce []byte
	var algorithm string
	var reentry bool
	err := p.db.QueryRowContext(ctx, `SELECT ciphertext,nonce,algorithm,requires_reentry FROM secrets WHERE id=?`, id).Scan(&ciphertext, &nonce, &algorithm, &reentry)
	if err != nil {
		return nil, err
	}
	if reentry {
		return nil, errSecretRequiresReentry
	}
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return nil, errSecretNotConfigured
	}
	if algorithm != operationalSecretAlgorithm {
		return nil, fmt.Errorf("unsupported secret algorithm %q", algorithm)
	}
	return p.decrypt(ciphertext, nonce)
}

func (p *databaseSecretProvider) GetSecretMetadata(ctx context.Context, id int64) (providers.SecretMetadata, error) {
	return p.metadata(ctx, id)
}

func (p *databaseSecretProvider) DeleteSecret(ctx context.Context, id int64) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM secrets WHERE id=?`, id)
	return err
}

func secretMetadataStatus(metadata providers.SecretMetadata) string {
	if metadata.RequiresReentry {
		return "requires_reentry"
	}
	return metadata.Status
}

func (s *server) credentialStatus(ctx context.Context, id sql.NullInt64) string {
	if !id.Valid || id.Int64 == 0 {
		return "missing"
	}
	metadata, err := s.secrets.GetSecretMetadata(ctx, id.Int64)
	if err != nil {
		return "missing"
	}
	return secretMetadataStatus(metadata)
}

func (s *server) decryptCredentialForIntegration(ctx context.Context, id sql.NullInt64) error {
	if !id.Valid || id.Int64 == 0 {
		return errSecretNotConfigured
	}
	_, err := s.secrets.DecryptSecretForAuthorizedUse(ctx, id.Int64)
	return err
}

func nullableSecretUpdated(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
