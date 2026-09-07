-- Demo v0.3.3: forward-only usability and governance evolution.
-- Existing Models, Provider Models, Profiles, Runtimes and Knowledge Items
-- remain the source of truth; the additions below only express new state.

ALTER TABLE profiles
  ADD COLUMN auxiliary_model_ids JSON NULL AFTER model_id,
  ADD COLUMN optional_skill_ids JSON NULL AFTER auxiliary_model_ids,
  ADD COLUMN knowledge_override_ids JSON NULL AFTER optional_skill_ids;
UPDATE profiles
  SET auxiliary_model_ids=JSON_ARRAY(), optional_skill_ids=JSON_ARRAY(), knowledge_override_ids=JSON_ARRAY()
  WHERE auxiliary_model_ids IS NULL OR optional_skill_ids IS NULL OR knowledge_override_ids IS NULL;
ALTER TABLE profiles
  MODIFY COLUMN auxiliary_model_ids JSON NOT NULL,
  MODIFY COLUMN optional_skill_ids JSON NOT NULL,
  MODIFY COLUMN knowledge_override_ids JSON NOT NULL;

ALTER TABLE runtimes
  ADD COLUMN observed_cpu_limit VARCHAR(40) NOT NULL DEFAULT '' AFTER actual_cpu,
  ADD COLUMN observed_memory_limit VARCHAR(40) NOT NULL DEFAULT '' AFTER actual_memory,
  ADD COLUMN observed_storage_limit VARCHAR(40) NOT NULL DEFAULT '' AFTER actual_storage,
  ADD COLUMN restart_required BOOLEAN NOT NULL DEFAULT FALSE AFTER observed_storage_limit;
UPDATE runtimes
  SET observed_cpu_limit=COALESCE(NULLIF(actual_cpu,''),cpu_limit),
      observed_memory_limit=COALESCE(NULLIF(actual_memory,''),memory_limit),
      observed_storage_limit=COALESCE(NULLIF(actual_storage,''),storage_limit)
  WHERE observed_cpu_limit='' OR observed_memory_limit='' OR observed_storage_limit='';

ALTER TABLE chat_conversations
  ADD COLUMN model_override_id BIGINT NULL AFTER profile_id,
  ADD CONSTRAINT fk_chat_conversation_model_override FOREIGN KEY (model_override_id) REFERENCES models(id) ON DELETE SET NULL;
ALTER TABLE chat_messages
  ADD COLUMN regenerated_from_message_id BIGINT NULL AFTER metadata,
  ADD CONSTRAINT fk_chat_message_regenerated_from FOREIGN KEY (regenerated_from_message_id) REFERENCES chat_messages(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS message_feedback (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  message_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  rating VARCHAR(16) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_message_feedback_user (message_id,user_id),
  CONSTRAINT fk_message_feedback_message FOREIGN KEY (message_id) REFERENCES chat_messages(id) ON DELETE CASCADE,
  CONSTRAINT fk_message_feedback_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS skill_review_workflows (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  organization_id BIGINT NOT NULL,
  name VARCHAR(160) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  description VARCHAR(800) NOT NULL DEFAULT '',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_skill_review_workflow_name (organization_id,name),
  CONSTRAINT fk_skill_review_workflow_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_skill_review_workflow_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS skill_review_workflow_steps (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  workflow_id BIGINT NOT NULL,
  step_order INT NOT NULL,
  name VARCHAR(160) NOT NULL,
  required_role_id BIGINT NULL,
  approval_mode VARCHAR(32) NOT NULL DEFAULT 'manual',
  required_approval BOOLEAN NOT NULL DEFAULT TRUE,
  UNIQUE KEY uq_skill_review_workflow_step (workflow_id,step_order),
  CONSTRAINT fk_skill_review_workflow_step_workflow FOREIGN KEY (workflow_id) REFERENCES skill_review_workflows(id) ON DELETE CASCADE,
  CONSTRAINT fk_skill_review_workflow_step_role FOREIGN KEY (required_role_id) REFERENCES roles(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS skill_review_instances (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  submission_id BIGINT NOT NULL UNIQUE,
  workflow_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at TIMESTAMP NULL,
  CONSTRAINT fk_skill_review_instance_submission FOREIGN KEY (submission_id) REFERENCES skill_submissions(id) ON DELETE CASCADE,
  CONSTRAINT fk_skill_review_instance_workflow FOREIGN KEY (workflow_id) REFERENCES skill_review_workflows(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS skill_review_steps (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  instance_id BIGINT NOT NULL,
  workflow_step_id BIGINT NULL,
  step_order INT NOT NULL,
  name VARCHAR(160) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  reviewer_id BIGINT NULL,
  started_at TIMESTAMP NULL,
  completed_at TIMESTAMP NULL,
  decision VARCHAR(32) NOT NULL DEFAULT '',
  comment TEXT NULL,
  risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
  findings JSON NOT NULL,
  UNIQUE KEY uq_skill_review_instance_step (instance_id,step_order),
  CONSTRAINT fk_skill_review_step_instance FOREIGN KEY (instance_id) REFERENCES skill_review_instances(id) ON DELETE CASCADE,
  CONSTRAINT fk_skill_review_step_template FOREIGN KEY (workflow_step_id) REFERENCES skill_review_workflow_steps(id) ON DELETE SET NULL,
  CONSTRAINT fk_skill_review_step_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS knowledge_import_jobs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  knowledge_base_id BIGINT NOT NULL,
  type VARCHAR(32) NOT NULL,
  file_count INT NOT NULL DEFAULT 0,
  success_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  duplicate_count INT NOT NULL DEFAULT 0,
  created_by BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  metadata JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at TIMESTAMP NULL,
  CONSTRAINT fk_knowledge_import_job_kb FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_import_job_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS knowledge_user_policies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  knowledge_base_id BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL DEFAULT 'organization',
  department_id BIGINT NULL,
  role_id BIGINT NULL,
  user_id BIGINT NULL,
  access_level VARCHAR(32) NOT NULL DEFAULT 'query_only',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_knowledge_user_policy (knowledge_base_id,scope,department_id,role_id,user_id),
  CONSTRAINT fk_knowledge_user_policy_kb FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_user_policy_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_user_policy_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_user_policy_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_knowledge_user_policy_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);
