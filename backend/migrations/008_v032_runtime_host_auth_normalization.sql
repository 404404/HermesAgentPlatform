-- Existing v0.3 demo hosts used an implementation-only `secret_reference`
-- auth type. v0.3.2 makes SSH Password the currently supported onboarding
-- mode while retaining the same credential_reference_id relationship.
UPDATE runtime_hosts SET auth_type='password' WHERE auth_type='secret_reference';
