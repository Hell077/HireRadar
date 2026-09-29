-- +goose Up
-- Recompute all saved candidates after matching preselection starts comparing
-- employment labels with the same punctuation-insensitive rules as the scorer.
INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload)
SELECT gen_random_uuid(), 'profile.changed', 'user', u.id::text,
       jsonb_build_object('user_id', u.id::text, 'reason', 'employment_normalization_refresh')
FROM users u
WHERE u.status = 'active'
  AND (
      EXISTS (SELECT 1 FROM user_profiles p WHERE p.user_id = u.id)
      OR EXISTS (SELECT 1 FROM user_skills s WHERE s.user_id = u.id)
      OR EXISTS (SELECT 1 FROM user_positions p WHERE p.user_id = u.id)
      OR EXISTS (SELECT 1 FROM job_preferences j WHERE j.user_id = u.id)
  );

-- +goose Down
DELETE FROM outbox_events
WHERE event_type = 'profile.changed'
  AND payload->>'reason' = 'employment_normalization_refresh';
