-- v0.3.3 Completion Hotfix: preserve old policy rows before normalising
-- nullable uniqueness, and add snapshots for review, installs and chat candidates.

CREATE TABLE IF NOT EXISTS knowledge_user_policies_011_backup AS
  SELECT * FROM knowledge_user_policies WHERE 1=0;
INSERT INTO knowledge_user_policies_011_backup
  SELECT p.* FROM knowledge_user_policies p
  WHERE NOT EXISTS (SELECT 1 FROM knowledge_user_policies_011_backup b WHERE b.id=p.id);

ALTER TABLE knowledge_user_policies
  ADD COLUMN scope_subject_key VARCHAR(96) NULL AFTER scope;
UPDATE knowledge_user_policies
  SET scope_subject_key=CASE
    WHEN scope='organization' THEN 'organization'
    WHEN scope='department' AND department_id IS NOT NULL THEN CONCAT('department:',department_id)
    WHEN scope='role' AND role_id IS NOT NULL THEN CONCAT('role:',role_id)
    WHEN scope='user' AND user_id IS NOT NULL THEN CONCAT('user:',user_id)
    ELSE CONCAT('legacy:',id)
  END
  WHERE scope_subject_key IS NULL OR scope_subject_key='';

CREATE TABLE IF NOT EXISTS knowledge_user_policy_011_conflicts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  old_policy_id BIGINT NOT NULL,
  kept_policy_id BIGINT NOT NULL,
  knowledge_base_id BIGINT NOT NULL,
  scope_subject_key VARCHAR(96) NOT NULL,
  reason VARCHAR(160) NOT NULL,
  captured_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_knowledge_policy_011_conflict (old_policy_id)
);
INSERT IGNORE INTO knowledge_user_policy_011_conflicts(old_policy_id,kept_policy_id,knowledge_base_id,scope_subject_key,reason)
  SELECT p.id,
    (SELECT winner.id FROM knowledge_user_policies winner
      WHERE winner.knowledge_base_id=p.knowledge_base_id AND winner.scope_subject_key=p.scope_subject_key
      ORDER BY winner.updated_at DESC,winner.id DESC LIMIT 1),
    p.knowledge_base_id,p.scope_subject_key,'latest updated policy retained during v0.3.3 completion migration'
  FROM knowledge_user_policies p
  WHERE p.id<>(SELECT winner.id FROM knowledge_user_policies winner
      WHERE winner.knowledge_base_id=p.knowledge_base_id AND winner.scope_subject_key=p.scope_subject_key
      ORDER BY winner.updated_at DESC,winner.id DESC LIMIT 1);
DELETE p FROM knowledge_user_policies p
  JOIN knowledge_user_policy_011_conflicts c ON c.old_policy_id=p.id;
ALTER TABLE knowledge_user_policies
  MODIFY COLUMN scope_subject_key VARCHAR(96) NOT NULL,
  DROP INDEX uq_knowledge_user_policy,
  ADD UNIQUE KEY uq_knowledge_user_policy_subject (knowledge_base_id,scope_subject_key);

ALTER TABLE skill_review_instances
  ADD COLUMN organization_id BIGINT NULL AFTER submission_id,
  ADD COLUMN skill_version_id BIGINT NULL AFTER workflow_id,
  ADD COLUMN workflow_snapshot JSON NULL AFTER skill_version_id;
UPDATE skill_review_instances i
  JOIN skill_submissions ss ON ss.id=i.submission_id
  JOIN users u ON u.id=ss.submitted_by
  SET i.organization_id=u.organization_id,i.skill_version_id=ss.skill_version_id
  WHERE i.organization_id IS NULL;

ALTER TABLE skill_review_steps
  ADD COLUMN required_role_id BIGINT NULL AFTER workflow_step_id,
  ADD COLUMN approval_mode VARCHAR(32) NOT NULL DEFAULT 'manual' AFTER required_role_id,
  ADD COLUMN required_approval BOOLEAN NOT NULL DEFAULT TRUE AFTER approval_mode,
  ADD COLUMN mock_source VARCHAR(96) NOT NULL DEFAULT '' AFTER findings;

CREATE TABLE IF NOT EXISTS profile_skill_installs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  profile_id BIGINT NOT NULL,
  skill_id BIGINT NOT NULL,
  skill_version_id BIGINT NULL,
  approval_request_id BIGINT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'installed',
  applied_at TIMESTAMP NULL,
  created_by BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_profile_skill_install (profile_id,skill_id),
  KEY idx_profile_skill_install_approval (approval_request_id),
  CONSTRAINT fk_profile_skill_install_profile FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_skill_install_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
  CONSTRAINT fk_profile_skill_install_version FOREIGN KEY (skill_version_id) REFERENCES skill_versions(id) ON DELETE SET NULL,
  CONSTRAINT fk_profile_skill_install_approval FOREIGN KEY (approval_request_id) REFERENCES approval_requests(id) ON DELETE SET NULL,
  CONSTRAINT fk_profile_skill_install_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE chat_messages
  ADD COLUMN selected_for_context BOOLEAN NOT NULL DEFAULT TRUE AFTER regenerated_from_message_id;

ALTER TABLE profiles
  ADD COLUMN runtime_id BIGINT NULL AFTER user_id,
  ADD KEY idx_profile_runtime (runtime_id),
  ADD CONSTRAINT fk_profile_runtime FOREIGN KEY (runtime_id) REFERENCES runtimes(id) ON DELETE SET NULL;
UPDATE profiles p
  JOIN runtimes r ON r.user_id=p.user_id
  SET p.runtime_id=r.id
  WHERE p.runtime_id IS NULL;
