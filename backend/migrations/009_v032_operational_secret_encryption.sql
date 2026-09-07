-- v0.3.2 Completion Hotfix: operational credentials must be decryptable by
-- an authorized backend integration. User login passwords remain in
-- users.password_hash and are intentionally not part of this migration.
ALTER TABLE secrets
  ADD COLUMN ciphertext MEDIUMBLOB NULL AFTER encrypted_value,
  ADD COLUMN nonce VARBINARY(32) NULL AFTER ciphertext,
  ADD COLUMN algorithm VARCHAR(64) NOT NULL DEFAULT '' AFTER nonce,
  ADD COLUMN key_version VARCHAR(64) NOT NULL DEFAULT '' AFTER algorithm,
  ADD COLUMN requires_reentry BOOLEAN NOT NULL DEFAULT FALSE AFTER key_version;

-- Existing values were bcrypt hashes written by the v0.3.2 onboarding UI.
-- They cannot be recovered, so deliberately clear them and require an
-- administrator to provide the credential again. This is safer than
-- presenting a misleading "configured" status.
UPDATE secrets
SET encrypted_value=NULL,
    ciphertext=NULL,
    nonce=NULL,
    algorithm='legacy-bcrypt',
    key_version='legacy',
    requires_reentry=TRUE,
    status='requires_reentry'
WHERE encrypted_value IS NOT NULL AND ciphertext IS NULL;
