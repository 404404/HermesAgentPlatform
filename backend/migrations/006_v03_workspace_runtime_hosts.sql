-- Demo v0.3: unified workspace and runtime infrastructure boundaries.
-- Existing tables remain the source of truth for users, profiles, runtimes,
-- templates, models, skills, knowledge and control-plane audit events.

ALTER TABLE users
  ADD COLUMN system_account BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE model_providers
  ADD COLUMN health_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  ADD COLUMN last_tested_at TIMESTAMP NULL,
  ADD COLUMN last_sync_at TIMESTAMP NULL;

CREATE TABLE IF NOT EXISTS provider_models (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  provider_id BIGINT NOT NULL,
  upstream_model VARCHAR(240) NOT NULL,
  display_name VARCHAR(240) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  sync_status VARCHAR(32) NOT NULL DEFAULT 'mock',
  last_sync_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_provider_upstream_model (provider_id, upstream_model),
  CONSTRAINT fk_provider_model_provider FOREIGN KEY (provider_id) REFERENCES model_providers(id) ON DELETE CASCADE
);

ALTER TABLE models
  ADD COLUMN provider_id BIGINT NULL,
  ADD COLUMN provider_model_id BIGINT NULL,
  ADD COLUMN purpose VARCHAR(32) NOT NULL DEFAULT 'main';

ALTER TABLE models
  ADD CONSTRAINT fk_model_provider FOREIGN KEY (provider_id) REFERENCES model_providers(id) ON DELETE SET NULL,
  ADD CONSTRAINT fk_model_provider_model FOREIGN KEY (provider_model_id) REFERENCES provider_models(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS model_slot_policies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  slot VARCHAR(64) NOT NULL,
  default_model_id BIGINT NULL,
  override_mode VARCHAR(32) NOT NULL DEFAULT 'admin_managed',
  allowed_models JSON NOT NULL,
  allowed_providers JSON NOT NULL,
  updated_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_model_slot_policy (organization_id, slot),
  CONSTRAINT fk_model_slot_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_model_slot_default FOREIGN KEY (default_model_id) REFERENCES models(id) ON DELETE SET NULL,
  CONSTRAINT fk_model_slot_updater FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS user_self_service_policies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL,
  department_id BIGINT NULL,
  role_id BIGINT NULL,
  user_id BIGINT NULL,
  capability VARCHAR(80) NOT NULL,
  mode VARCHAR(32) NOT NULL DEFAULT 'disabled',
  allowed_values JSON NOT NULL,
  updated_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_self_service_policy (organization_id, scope, department_id, role_id, user_id, capability),
  CONSTRAINT fk_self_service_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_self_service_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_self_service_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_self_service_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_self_service_updater FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS channel_policies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  channel_type VARCHAR(64) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  user_self_service VARCHAR(32) NOT NULL DEFAULT 'disabled',
  user_credentials_allowed BOOLEAN NOT NULL DEFAULT FALSE,
  admin_managed BOOLEAN NOT NULL DEFAULT TRUE,
  policy JSON NOT NULL,
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_channel_policy (organization_id, channel_type),
  CONSTRAINT fk_channel_policy_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_channel_policy_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS channel_connections (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  profile_id BIGINT NULL,
  channel_type VARCHAR(64) NOT NULL,
  credential_reference_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'disconnected',
  settings JSON NOT NULL,
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_channel_connection (user_id, profile_id, channel_type),
  CONSTRAINT fk_channel_connection_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_channel_connection_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_channel_connection_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
  CONSTRAINT fk_channel_connection_secret FOREIGN KEY (credential_reference_id) REFERENCES secrets(id) ON DELETE SET NULL,
  CONSTRAINT fk_channel_connection_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS chat_conversations (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  profile_id BIGINT NOT NULL,
  title VARCHAR(240) NOT NULL DEFAULT 'New conversation',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_chat_conversation_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_chat_conversation_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_chat_conversation_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  conversation_id BIGINT NOT NULL,
  role VARCHAR(32) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  metadata JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_chat_message_conversation FOREIGN KEY (conversation_id) REFERENCES chat_conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS runtime_hosts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  name VARCHAR(160) NOT NULL,
  hostname VARCHAR(255) NOT NULL,
  address VARCHAR(255) NOT NULL,
  ssh_port INT NOT NULL DEFAULT 22,
  auth_type VARCHAR(32) NOT NULL DEFAULT 'secret_reference',
  credential_reference_id BIGINT NULL,
  docker_endpoint VARCHAR(255) NOT NULL DEFAULT 'mock://local-runtime-provider',
  docker_version VARCHAR(80) NOT NULL DEFAULT 'mock',
  cpu_total VARCHAR(40) NOT NULL DEFAULT '0 CPU',
  memory_total VARCHAR(40) NOT NULL DEFAULT '0 GB',
  storage_total VARCHAR(40) NOT NULL DEFAULT '0 GB',
  cpu_allocated VARCHAR(40) NOT NULL DEFAULT '0 CPU',
  memory_allocated VARCHAR(40) NOT NULL DEFAULT '0 GB',
  storage_allocated VARCHAR(40) NOT NULL DEFAULT '0 GB',
  runtime_count INT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  labels JSON NOT NULL,
  last_seen TIMESTAMP NULL,
  last_inventory_at TIMESTAMP NULL,
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_runtime_host_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_runtime_host_secret FOREIGN KEY (credential_reference_id) REFERENCES secrets(id) ON DELETE SET NULL,
  CONSTRAINT fk_runtime_host_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE runtimes
  ADD COLUMN host_id BIGINT NULL,
  ADD COLUMN container_name VARCHAR(160) NOT NULL DEFAULT '',
  ADD COLUMN actual_cpu VARCHAR(40) NOT NULL DEFAULT '',
  ADD COLUMN actual_memory VARCHAR(40) NOT NULL DEFAULT '',
  ADD COLUMN actual_storage VARCHAR(40) NOT NULL DEFAULT '',
  ADD COLUMN observed_image_version VARCHAR(80) NOT NULL DEFAULT '',
  ADD COLUMN placement_status VARCHAR(32) NOT NULL DEFAULT 'unplaced';

ALTER TABLE runtimes
  ADD CONSTRAINT fk_runtime_host FOREIGN KEY (host_id) REFERENCES runtime_hosts(id) ON DELETE SET NULL;

CREATE INDEX idx_chat_conversation_user ON chat_conversations (user_id, updated_at);
CREATE INDEX idx_chat_message_conversation ON chat_messages (conversation_id, created_at);
CREATE INDEX idx_runtime_host_status ON runtime_hosts (organization_id, status);
CREATE INDEX idx_self_service_lookup ON user_self_service_policies (organization_id, capability, scope);
