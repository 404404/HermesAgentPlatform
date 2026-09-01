ALTER TABLE departments
  ADD COLUMN code VARCHAR(80) NOT NULL DEFAULT '',
  ADD COLUMN manager_user_id BIGINT NULL,
  ADD COLUMN default_runtime_template_id BIGINT NULL;

ALTER TABLE profile_templates
  ADD COLUMN template_version INT NOT NULL DEFAULT 1,
  ADD COLUMN skill_policies JSON NULL;
UPDATE profile_templates SET skill_policies=JSON_ARRAY() WHERE skill_policies IS NULL;
ALTER TABLE profile_templates MODIFY COLUMN skill_policies JSON NOT NULL;

ALTER TABLE profile_template_bindings
  ADD COLUMN user_id BIGINT NULL;

ALTER TABLE profiles
  ADD COLUMN source_template_version INT NULL,
  ADD COLUMN assignment_sources JSON NULL;
UPDATE profiles SET assignment_sources=JSON_ARRAY() WHERE assignment_sources IS NULL;
ALTER TABLE profiles MODIFY COLUMN assignment_sources JSON NOT NULL;

ALTER TABLE runtimes
  ADD COLUMN desired_status VARCHAR(32) NOT NULL DEFAULT 'running',
  ADD COLUMN observed_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  ADD COLUMN kill_switch_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN kill_switch_reason VARCHAR(1200) NOT NULL DEFAULT '',
  ADD COLUMN kill_switched_at TIMESTAMP NULL;
UPDATE runtimes SET desired_status=CASE WHEN status IN ('running','stopped','suspended') THEN status ELSE 'running' END,observed_status=CASE WHEN status IN ('running','stopped','suspended') THEN status ELSE 'unknown' END;

ALTER TABLE audit_logs
  ADD COLUMN category VARCHAR(64) NOT NULL DEFAULT 'Security',
  ADD COLUMN action_label VARCHAR(200) NOT NULL DEFAULT '',
  ADD COLUMN profile_id BIGINT NULL;

CREATE TABLE IF NOT EXISTS profile_assignment_sources (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  profile_id BIGINT NOT NULL,
  template_id BIGINT NOT NULL,
  source_type VARCHAR(32) NOT NULL,
  source_id BIGINT NULL,
  source_label VARCHAR(240) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_profile_assignment_source (profile_id, template_id, source_type, source_id),
  CONSTRAINT fk_assignment_source_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
  CONSTRAINT fk_assignment_source_template FOREIGN KEY (template_id) REFERENCES profile_templates(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS knowledge_items (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  knowledge_base_id BIGINT NOT NULL,
  type VARCHAR(32) NOT NULL,
  title VARCHAR(240) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  question MEDIUMTEXT NOT NULL,
  answer MEDIUMTEXT NOT NULL,
  purpose VARCHAR(800) NOT NULL,
  prerequisites MEDIUMTEXT NOT NULL,
  steps JSON NOT NULL,
  notes MEDIUMTEXT NOT NULL,
  tags JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  owner_user_id BIGINT NULL,
  version INT NOT NULL DEFAULT 1,
  index_status VARCHAR(32) NOT NULL DEFAULT 'not_indexed',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_knowledge_item_base FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_item_owner FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS knowledge_item_versions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  item_id BIGINT NOT NULL,
  version INT NOT NULL,
  payload JSON NOT NULL,
  content_hash CHAR(64) NOT NULL DEFAULT '',
  created_by BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_knowledge_item_version (item_id, version),
  CONSTRAINT fk_knowledge_item_version_item FOREIGN KEY (item_id) REFERENCES knowledge_items(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_item_version_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS executions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  execution_id VARCHAR(160) NOT NULL UNIQUE,
  organization_id BIGINT NOT NULL,
  department_id BIGINT NULL,
  user_id BIGINT NOT NULL,
  profile_id BIGINT NULL,
  runtime_id BIGINT NULL,
  model_id BIGINT NULL,
  skill_ids JSON NOT NULL,
  tool_names JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'queued',
  started_at TIMESTAMP NULL,
  finished_at TIMESTAMP NULL,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  input_tokens BIGINT NOT NULL DEFAULT 0,
  output_tokens BIGINT NOT NULL DEFAULT 0,
  cost DECIMAL(14,6) NOT NULL DEFAULT 0,
  risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
  risk_score DECIMAL(5,2) NOT NULL DEFAULT 0,
  risk_reason VARCHAR(1200) NOT NULL DEFAULT '',
  requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
  approval_request_id BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_execution_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE SET NULL,
  CONSTRAINT fk_execution_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE SET NULL,
  CONSTRAINT fk_execution_runtime FOREIGN KEY (runtime_id) REFERENCES runtimes(id) ON DELETE SET NULL,
  CONSTRAINT fk_execution_model FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE SET NULL,
  CONSTRAINT fk_execution_approval FOREIGN KEY (approval_request_id) REFERENCES approval_requests(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS execution_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  execution_id BIGINT NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  message VARCHAR(1200) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_execution_event_execution FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS runtime_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  runtime_id BIGINT NOT NULL,
  event_type VARCHAR(80) NOT NULL,
  message VARCHAR(1200) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_runtime_events_runtime FOREIGN KEY (runtime_id) REFERENCES runtimes(id) ON DELETE CASCADE
);

CREATE INDEX idx_execution_user_created ON executions (user_id, created_at);
CREATE INDEX idx_execution_risk_status ON executions (risk_level, status, created_at);
CREATE INDEX idx_knowledge_item_type_status ON knowledge_items (knowledge_base_id, type, status);
CREATE INDEX idx_profile_assignment_template ON profile_assignment_sources (template_id, source_type);
