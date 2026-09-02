-- Demo v0.2.2: forward-only relationship metadata.
-- Runtime templates own infrastructure policy bindings; Agent Templates own
-- model/skill/knowledge behavior.  These tables deliberately do not connect
-- runtime templates to agent capabilities.
CREATE TABLE IF NOT EXISTS runtime_template_bindings (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  runtime_template_id BIGINT NOT NULL,
  binding_type VARCHAR(32) NOT NULL,
  role_id BIGINT NULL,
  department_id BIGINT NULL,
  binding_priority INT NOT NULL DEFAULT 0,
  policy VARCHAR(32) NOT NULL DEFAULT 'default',
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_runtime_template_binding (runtime_template_id, binding_type, role_id, department_id),
  CONSTRAINT fk_runtime_policy_template FOREIGN KEY (runtime_template_id) REFERENCES runtime_templates(id) ON DELETE CASCADE,
  CONSTRAINT fk_runtime_policy_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_runtime_policy_department FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE CASCADE,
  CONSTRAINT fk_runtime_policy_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE knowledge_bindings
  ADD COLUMN agent_template_id BIGINT NULL,
  ADD CONSTRAINT fk_knowledge_binding_agent_template FOREIGN KEY (agent_template_id) REFERENCES profile_templates(id) ON DELETE CASCADE;

CREATE INDEX idx_runtime_policy_role ON runtime_template_bindings (role_id, binding_priority);
CREATE INDEX idx_runtime_policy_department ON runtime_template_bindings (department_id, binding_priority);
CREATE INDEX idx_knowledge_binding_agent_template ON knowledge_bindings (agent_template_id);
