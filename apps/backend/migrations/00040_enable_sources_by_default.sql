-- +goose Up
-- Earlier settings pages rendered every source unchecked when a user had no
-- saved preferences. Saving settings then persisted an all-disabled list,
-- which prevented matching from considering any jobs. Treat that legacy
-- state as an unset preference and enable the sources for those users.
WITH all_disabled_users AS (
    SELECT user_id
    FROM user_source_preferences
    GROUP BY user_id
    HAVING NOT bool_or(enabled)
), enabled_preferences AS (
    UPDATE user_source_preferences p
       SET enabled = true
      FROM all_disabled_users u
     WHERE p.user_id = u.user_id AND NOT p.enabled
    RETURNING p.user_id
), affected_users AS (
    SELECT DISTINCT user_id FROM enabled_preferences
)
INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload)
SELECT gen_random_uuid(), 'profile.changed', 'user', user_id::text,
       jsonb_build_object('user_id', user_id::text, 'reason', 'default_sources_enabled')
FROM affected_users;

-- +goose Down
-- Keep the user's now-enabled selections if this data backfill is rolled back.
SELECT 1;
