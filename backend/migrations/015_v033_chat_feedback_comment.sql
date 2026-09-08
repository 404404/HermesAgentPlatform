-- The existing feedback table retained only a rating.  v0.3.3 persists the
-- optional user comment as well, without changing historic feedback rows.
ALTER TABLE message_feedback
  ADD COLUMN comment TEXT NULL AFTER rating;
