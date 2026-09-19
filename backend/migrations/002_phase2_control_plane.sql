CREATE TABLE IF NOT EXISTS skill_artifacts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  skill_version_id BIGINT NOT NULL UNIQUE,
  artifact_hash VARCHAR(128) NOT NULL DEFAULT '',
  size_bytes BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_artifact_version FOREIGN KEY (skill_version_id) REFERENCES skill_versions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS skill_artifact_files (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  artifact_id BIGINT NOT NULL,
  path VARCHAR(500) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  content_type VARCHAR(120) NOT NULL DEFAULT 'text/plain',
  size_bytes BIGINT NOT NULL DEFAULT 0,
  sha256 CHAR(64) NOT NULL DEFAULT '',
  is_directory BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_artifact_file_path (artifact_id, path),
  CONSTRAINT fk_artifact_file_artifact FOREIGN KEY (artifact_id) REFERENCES skill_artifacts(id) ON DELETE CASCADE
);

ALTER TABLE skill_versions
  ADD COLUMN immutable BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN risk_level VARCHAR(32) NOT NULL DEFAULT 'low';

CREATE TABLE IF NOT EXISTS knowledge_documents (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  knowledge_base_id BIGINT NOT NULL,
  title VARCHAR(240) NOT NULL,
  type VARCHAR(32) NOT NULL DEFAULT 'markdown',
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  owner_user_id BIGINT NULL,
  index_status VARCHAR(32) NOT NULL DEFAULT 'not_indexed',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_document_kb FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  CONSTRAINT fk_document_owner FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS knowledge_document_versions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  document_id BIGINT NOT NULL,
  version INT NOT NULL,
  content MEDIUMTEXT NOT NULL,
  content_hash CHAR(64) NOT NULL DEFAULT '',
  created_by BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_document_version (document_id, version),
  CONSTRAINT fk_document_version_document FOREIGN KEY (document_id) REFERENCES knowledge_documents(id) ON DELETE CASCADE,
  CONSTRAINT fk_document_version_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE knowledge_bindings
  ADD COLUMN organization_id BIGINT NULL,
  ADD COLUMN policy VARCHAR(64) NOT NULL DEFAULT 'allow',
  ADD COLUMN created_by BIGINT NULL,
  ADD CONSTRAINT fk_kb_binding_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  ADD CONSTRAINT fk_kb_binding_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS runtime_templates (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  name VARCHAR(120) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  cpu_limit VARCHAR(40) NOT NULL DEFAULT '1 CPU',
  memory_limit VARCHAR(40) NOT NULL DEFAULT '1 GB',
  storage_limit VARCHAR(40) NOT NULL DEFAULT '10 GB',
  profile_limit INT NOT NULL DEFAULT 5,
  max_concurrent_jobs INT NOT NULL DEFAULT 2,
  image_version VARCHAR(80) NOT NULL DEFAULT 'mock-hermes-0.2',
  runtime_provider VARCHAR(80) NOT NULL DEFAULT 'mock',
  runtime_class VARCHAR(80) NOT NULL DEFAULT 'standard',
  network_policy VARCHAR(80) NOT NULL DEFAULT 'restricted',
  auto_start BOOLEAN NOT NULL DEFAULT TRUE,
  auto_suspend BOOLEAN NOT NULL DEFAULT FALSE,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_runtime_template_name (organization_id, name),
  CONSTRAINT fk_template_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_template_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE runtimes
  ADD COLUMN template_id BIGINT NULL,
  ADD COLUMN storage_limit VARCHAR(40) NOT NULL DEFAULT '10 GB',
  ADD COLUMN profile_limit INT NOT NULL DEFAULT 5,
  ADD COLUMN max_concurrent_jobs INT NOT NULL DEFAULT 2,
  ADD COLUMN image_version VARCHAR(80) NOT NULL DEFAULT 'mock-hermes-0.2',
  ADD COLUMN runtime_provider VARCHAR(80) NOT NULL DEFAULT 'mock',
  ADD COLUMN runtime_class VARCHAR(80) NOT NULL DEFAULT 'standard',
  ADD COLUMN network_policy VARCHAR(80) NOT NULL DEFAULT 'restricted',
  ADD COLUMN auto_start BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN auto_suspend BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  ADD CONSTRAINT fk_runtime_template FOREIGN KEY (template_id) REFERENCES runtime_templates(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS profile_templates (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  name VARCHAR(120) NOT NULL,
  display_name VARCHAR(160) NOT NULL,
  description VARCHAR(800) NOT NULL DEFAULT '',
  default_model_id BIGINT NULL,
  runtime_class VARCHAR(80) NOT NULL DEFAULT 'standard',
  default_skills JSON NOT NULL,
  default_knowledge JSON NOT NULL,
  managed BOOLEAN NOT NULL DEFAULT TRUE,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_profile_template_name (organization_id, name),
  CONSTRAINT fk_profile_template_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_template_model FOREIGN KEY (default_model_id) REFERENCES models(id) ON DELETE SET NULL,
  CONSTRAINT fk_profile_template_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS profile_template_bindings (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  template_id BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL,
  organization_id BIGINT NULL,
  department_id BIGINT NULL,
  role_id BIGINT NULL,
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_profile_template_binding_template FOREIGN KEY (template_id) REFERENCES profile_templates(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_template_binding_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_template_binding_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_template_binding_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_template_binding_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE profiles
  ADD COLUMN profile_type VARCHAR(32) NOT NULL DEFAULT 'personal',
  ADD COLUMN managed BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN source_template_id BIGINT NULL,
  ADD CONSTRAINT fk_profile_source_template FOREIGN KEY (source_template_id) REFERENCES profile_templates(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS model_providers (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  name VARCHAR(160) NOT NULL,
  type VARCHAR(64) NOT NULL DEFAULT 'custom',
  mode VARCHAR(64) NOT NULL DEFAULT 'hermes_native',
  base_url VARCHAR(500) NOT NULL DEFAULT '',
  auth_type VARCHAR(64) NOT NULL DEFAULT 'secret_reference',
  secret_reference_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  description VARCHAR(800) NOT NULL DEFAULT '',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_model_provider_name (organization_id, name),
  CONSTRAINT fk_model_provider_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_model_provider_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS secrets (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  name VARCHAR(160) NOT NULL,
  type VARCHAR(64) NOT NULL DEFAULT 'api_key',
  scope VARCHAR(32) NOT NULL DEFAULT 'organization',
  owner_user_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'not_configured',
  encrypted_value VARBINARY(4096) NULL,
  last_updated TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_secret_name (organization_id, name),
  CONSTRAINT fk_secret_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_secret_owner FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE model_providers
  ADD CONSTRAINT fk_model_provider_secret FOREIGN KEY (secret_reference_id) REFERENCES secrets(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS approval_requests (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  type VARCHAR(64) NOT NULL,
  requester BIGINT NOT NULL,
  resource_type VARCHAR(80) NOT NULL,
  resource_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  risk_level VARCHAR(32) NOT NULL DEFAULT 'medium',
  current_reviewer BIGINT NULL,
  reason VARCHAR(1200) NOT NULL DEFAULT '',
  metadata JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at TIMESTAMP NULL,
  CONSTRAINT fk_approval_requester FOREIGN KEY (requester) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_approval_reviewer FOREIGN KEY (current_reviewer) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS approval_steps (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  approval_request_id BIGINT NOT NULL,
  step_order INT NOT NULL DEFAULT 1,
  reviewer_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  comment VARCHAR(1200) NOT NULL DEFAULT '',
  resolved_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_approval_step_request FOREIGN KEY (approval_request_id) REFERENCES approval_requests(id) ON DELETE CASCADE,
  CONSTRAINT fk_approval_step_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE audit_logs
  ADD COLUMN user_agent VARCHAR(500) NOT NULL DEFAULT '',
  ADD COLUMN request_id VARCHAR(120) NOT NULL DEFAULT '',
  ADD COLUMN trace_id VARCHAR(120) NOT NULL DEFAULT '',
  ADD COLUMN risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
  ADD COLUMN risk_score DECIMAL(5,2) NOT NULL DEFAULT 0,
  ADD COLUMN risk_reason VARCHAR(1200) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS risk_rules (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(120) NOT NULL UNIQUE,
  name VARCHAR(200) NOT NULL,
  event_pattern VARCHAR(200) NOT NULL,
  risk_level VARCHAR(32) NOT NULL,
  risk_score DECIMAL(5,2) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS risk_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  audit_log_id BIGINT NULL,
  actor_user_id BIGINT NULL,
  action VARCHAR(160) NOT NULL,
  resource_type VARCHAR(80) NOT NULL,
  resource_id BIGINT NULL,
  risk_level VARCHAR(32) NOT NULL,
  risk_score DECIMAL(5,2) NOT NULL DEFAULT 0,
  risk_reason VARCHAR(1200) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'open',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_risk_audit FOREIGN KEY (audit_log_id) REFERENCES audit_logs(id) ON DELETE SET NULL,
  CONSTRAINT fk_risk_actor FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS system_settings (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  setting_key VARCHAR(160) NOT NULL,
  setting_value TEXT NOT NULL,
  value_type VARCHAR(32) NOT NULL DEFAULT 'string',
  updated_by BIGINT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_system_setting (organization_id, setting_key),
  CONSTRAINT fk_setting_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_setting_updater FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS resource_change_history (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  resource_type VARCHAR(80) NOT NULL,
  resource_id BIGINT NOT NULL,
  before_state JSON NOT NULL,
  after_state JSON NOT NULL,
  actor_user_id BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_change_actor FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS quota_policies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL,
  department_id BIGINT NULL,
  role_id BIGINT NULL,
  user_id BIGINT NULL,
  monthly_token_limit BIGINT NULL,
  monthly_cost_limit DECIMAL(14,4) NULL,
  max_profiles INT NULL,
  max_runtime_cpu VARCHAR(40) NULL,
  max_runtime_memory VARCHAR(40) NULL,
  max_concurrent_executions INT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_quota_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_quota_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_quota_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_quota_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_quota_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  type VARCHAR(64) NOT NULL,
  title VARCHAR(240) NOT NULL,
  body VARCHAR(1200) NOT NULL DEFAULT '',
  resource_type VARCHAR(80) NULL,
  resource_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'unread',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  read_at TIMESTAMP NULL,
  CONSTRAINT fk_notification_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_preferences (
  user_id BIGINT PRIMARY KEY,
  language VARCHAR(16) NOT NULL DEFAULT 'en-US',
  timezone VARCHAR(80) NOT NULL DEFAULT 'UTC',
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_preference_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_audit_created_risk ON audit_logs (created_at, risk_level);
CREATE INDEX idx_audit_actor ON audit_logs (actor_user_id);
CREATE INDEX idx_risk_created_status ON risk_events (created_at, status, risk_level);
CREATE INDEX idx_document_status ON knowledge_documents (knowledge_base_id, status, index_status);
CREATE INDEX idx_notification_user_status ON notifications (user_id, status, created_at);
