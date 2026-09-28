-- +goose Up
ALTER TABLE parsed_resumes
    ADD COLUMN languages jsonb NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(languages) = 'array');

-- Re-evaluate existing profiles against their stored CV analysis after the
-- matcher learns to use resume positions and languages.
INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload)
SELECT gen_random_uuid(), 'profile.changed', 'user', r.user_id::text,
       jsonb_build_object('user_id', r.user_id::text, 'reason', 'resume_analysis_refresh')
FROM resumes r
WHERE r.status = 'processed'
  AND EXISTS (SELECT 1 FROM parsed_resumes p WHERE p.resume_id = r.id)
GROUP BY r.user_id;

-- +goose Down
DELETE FROM outbox_events
WHERE event_type = 'profile.changed'
  AND payload->>'reason' = 'resume_analysis_refresh';
ALTER TABLE parsed_resumes DROP COLUMN languages;
