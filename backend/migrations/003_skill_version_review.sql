ALTER TABLE skill_submissions
  ADD COLUMN skill_version_id BIGINT NULL,
  ADD CONSTRAINT fk_submission_skill_version FOREIGN KEY (skill_version_id) REFERENCES skill_versions(id) ON DELETE SET NULL;
