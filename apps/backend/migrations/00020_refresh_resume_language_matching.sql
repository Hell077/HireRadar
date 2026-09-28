-- +goose Up
-- Recompute existing profiles after using a clearly dominant CV language as
-- a fallback when the document lacks a spoken-language section.
INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload)
SELECT gen_random_uuid(), 'profile.changed', 'user', u.id::text,
       jsonb_build_object('user_id', u.id::text, 'reason', 'resume_language_refresh')
FROM users u
WHERE u.status = 'active'
  AND EXISTS (
      SELECT 1 FROM resumes r
      WHERE r.user_id = u.id AND r.status = 'processed'
  );

-- +goose Down
DELETE FROM outbox_events
WHERE event_type = 'profile.changed'
  AND payload->>'reason' = 'resume_language_refresh';
