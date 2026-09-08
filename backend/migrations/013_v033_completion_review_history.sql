-- Completes review-step history fields without altering migration 010.

ALTER TABLE skill_review_steps
  ADD COLUMN decision_note TEXT NULL AFTER comment,
  ADD COLUMN reviewed_by BIGINT NULL AFTER reviewer_id,
  ADD COLUMN reviewed_at TIMESTAMP NULL AFTER completed_at,
  ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER findings;
