-- v0.3.3 Completion Hotfix follow-up: columns used by immutable review
-- instances and the mock-runtime application state.  Kept separate from 011
-- so an already-upgraded demo can evolve without rewriting history.

ALTER TABLE skill_review_instances
  ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER workflow_snapshot;

ALTER TABLE skill_review_steps
  ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER mock_source;

ALTER TABLE profile_skill_installs
  ADD COLUMN runtime_apply_status VARCHAR(40) NOT NULL DEFAULT 'not_applied' AFTER status;

-- This runs once at migration time, not on every server boot.  It gives each
-- documented demo user a policy-backed knowledge source while preserving any
-- existing administrator-selected access level through INSERT IGNORE.
INSERT IGNORE INTO knowledge_user_policies(knowledge_base_id,scope,scope_subject_key,user_id,access_level,created_by,created_at,updated_at)
  SELECT kb.id,'user',CONCAT('user:',u.id),u.id,
    CASE WHEN u.username='user02' THEN 'query_only' ELSE 'read' END,
    (SELECT id FROM users WHERE username='admin' LIMIT 1),NOW(),NOW()
  FROM knowledge_bases kb
  JOIN users u ON u.username IN ('user01','user02')
  WHERE kb.status='active';
