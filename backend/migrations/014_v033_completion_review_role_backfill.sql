-- Existing demo workflows created before reviewer roles were modeled receive a
-- deterministic role requirement.  Later administrator edits remain intact.

UPDATE skill_review_workflow_steps s
  SET required_role_id = CASE
    WHEN s.name LIKE '%Security%' THEN (SELECT id FROM roles WHERE name='Security Administrator' LIMIT 1)
    WHEN s.name LIKE '%Publish%' THEN (SELECT id FROM roles WHERE name='Audit Administrator' LIMIT 1)
    ELSE (SELECT id FROM roles WHERE name='System Administrator' LIMIT 1)
  END
  WHERE s.approval_mode='manual' AND s.required_role_id IS NULL;
