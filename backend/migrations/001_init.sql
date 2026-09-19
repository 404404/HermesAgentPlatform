CREATE TABLE IF NOT EXISTS organizations (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(160) NOT NULL,
  slug VARCHAR(160) NOT NULL UNIQUE,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS departments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  parent_id BIGINT NULL,
  name VARCHAR(160) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_department_name (organization_id, parent_id, name),
  CONSTRAINT fk_department_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_department_parent FOREIGN KEY (parent_id) REFERENCES departments(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  department_id BIGINT NULL,
  username VARCHAR(80) NOT NULL UNIQUE,
  display_name VARCHAR(160) NOT NULL,
  email VARCHAR(255) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  last_login_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_user_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_user_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS auth_identities (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  provider_type VARCHAR(32) NOT NULL,
  provider_id VARCHAR(120) NOT NULL,
  external_subject VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_auth_identity (provider_type, provider_id, external_subject),
  CONSTRAINT fk_identity_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS roles (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NULL,
  name VARCHAR(120) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  is_system BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_role_name (organization_id, name),
  CONSTRAINT fk_role_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS permissions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(160) NOT NULL UNIQUE,
  description VARCHAR(500) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id BIGINT NOT NULL,
  permission_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  CONSTRAINT fk_role_permission_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_role_permission_permission FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS role_bindings (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  role_id BIGINT NOT NULL,
  organization_id BIGINT NULL,
  department_id BIGINT NULL,
  user_id BIGINT NULL,
  profile_id BIGINT NULL,
  scope VARCHAR(32) NOT NULL DEFAULT 'global',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_binding_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_binding_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_binding_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_binding_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS models (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NULL,
  name VARCHAR(120) NOT NULL UNIQUE,
  display_name VARCHAR(160) NOT NULL,
  provider VARCHAR(120) NOT NULL DEFAULT 'model-gateway',
  upstream_model VARCHAR(160) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  description VARCHAR(500) NOT NULL DEFAULT '',
  cost_class VARCHAR(64) NOT NULL DEFAULT 'medium',
  data_classification VARCHAR(64) NOT NULL DEFAULT 'internal',
  user_selectable BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_model_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS profiles (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  model_id BIGINT NULL,
  name VARCHAR(120) NOT NULL,
  display_name VARCHAR(160) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  runtime_class VARCHAR(32) NOT NULL DEFAULT 'shared-user',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_profile_user_name (user_id, name),
  CONSTRAINT fk_profile_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_model FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS runtimes (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL UNIQUE,
  runtime_id VARCHAR(160) NOT NULL UNIQUE,
  status VARCHAR(32) NOT NULL DEFAULT 'stopped',
  provider VARCHAR(80) NOT NULL DEFAULT 'mock',
  hermes_version VARCHAR(80) NOT NULL DEFAULT 'unknown',
  cpu_limit VARCHAR(40) NOT NULL DEFAULT '1 CPU',
  memory_limit VARCHAR(40) NOT NULL DEFAULT '512Mi',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen TIMESTAMP NULL,
  CONSTRAINT fk_runtime_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS skills (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(120) NOT NULL UNIQUE,
  display_name VARCHAR(160) NOT NULL,
  description VARCHAR(800) NOT NULL DEFAULT '',
  category VARCHAR(120) NOT NULL DEFAULT 'General',
  publisher_id BIGINT NULL,
  status VARCHAR(40) NOT NULL DEFAULT 'draft',
  latest_version VARCHAR(40) NOT NULL DEFAULT '0.1.0',
  risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
  install_count INT NOT NULL DEFAULT 0,
  use_count INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_skill_publisher FOREIGN KEY (publisher_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS skill_versions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  skill_id BIGINT NOT NULL,
  version VARCHAR(40) NOT NULL,
  artifact_hash VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(40) NOT NULL DEFAULT 'draft',
  required_tools JSON NOT NULL,
  required_network JSON NOT NULL,
  required_secrets JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_skill_version (skill_id, version),
  CONSTRAINT fk_skill_version_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS skill_submissions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  skill_id BIGINT NOT NULL,
  submitted_by BIGINT NOT NULL,
  status VARCHAR(40) NOT NULL DEFAULT 'draft',
  notes VARCHAR(1200) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_submission_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
  CONSTRAINT fk_submission_user FOREIGN KEY (submitted_by) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS skill_reviews (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  submission_id BIGINT NOT NULL,
  reviewer_id BIGINT NOT NULL,
  decision VARCHAR(32) NOT NULL,
  comment VARCHAR(1200) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_review_submission FOREIGN KEY (submission_id) REFERENCES skill_submissions(id) ON DELETE CASCADE,
  CONSTRAINT fk_review_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS skill_assignments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  skill_id BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL,
  organization_id BIGINT NULL,
  department_id BIGINT NULL,
  role_id BIGINT NULL,
  user_id BIGINT NULL,
  profile_id BIGINT NULL,
  policy VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_assignment_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS knowledge_bases (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  owner_department_id BIGINT NULL,
  name VARCHAR(160) NOT NULL UNIQUE,
  description VARCHAR(800) NOT NULL DEFAULT '',
  visibility VARCHAR(32) NOT NULL DEFAULT 'department',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  document_count INT NOT NULL DEFAULT 0,
  last_indexed TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_kb_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_kb_department FOREIGN KEY (owner_department_id) REFERENCES departments(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS knowledge_bindings (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  knowledge_base_id BIGINT NOT NULL,
  binding_type VARCHAR(32) NOT NULL,
  department_id BIGINT NULL,
  role_id BIGINT NULL,
  profile_id BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_kb_binding_kb FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  CONSTRAINT fk_kb_binding_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_kb_binding_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_kb_binding_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS usage_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  department_id BIGINT NULL,
  user_id BIGINT NULL,
  profile_id BIGINT NULL,
  session_id VARCHAR(160) NOT NULL DEFAULT '',
  execution_id VARCHAR(160) NOT NULL DEFAULT '',
  model_id BIGINT NULL,
  skill_id BIGINT NULL,
  runtime_id BIGINT NULL,
  token_input BIGINT NOT NULL DEFAULT 0,
  token_output BIGINT NOT NULL DEFAULT 0,
  requests BIGINT NOT NULL DEFAULT 0,
  executions BIGINT NOT NULL DEFAULT 0,
  skill_calls BIGINT NOT NULL DEFAULT 0,
  tool_calls BIGINT NOT NULL DEFAULT 0,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_usage_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_usage_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE SET NULL,
  CONSTRAINT fk_usage_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
  CONSTRAINT fk_usage_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE SET NULL,
  CONSTRAINT fk_usage_model FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE SET NULL,
  CONSTRAINT fk_usage_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE SET NULL,
  CONSTRAINT fk_usage_runtime FOREIGN KEY (runtime_id) REFERENCES runtimes(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  actor_user_id BIGINT NULL,
  action VARCHAR(160) NOT NULL,
  resource_type VARCHAR(80) NOT NULL,
  resource_id BIGINT NULL,
  scope VARCHAR(80) NOT NULL DEFAULT 'global',
  result VARCHAR(32) NOT NULL DEFAULT 'success',
  ip_address VARCHAR(64) NOT NULL DEFAULT '',
  metadata JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_audit_actor FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL
);
