-- +goose Up
WITH normalized AS (
    SELECT j.id,
           COALESCE(ARRAY(
               SELECT DISTINCT mapped
               FROM unnest(j.employment_types) AS input(label)
               CROSS JOIN LATERAL (SELECT trim(regexp_replace(lower(input.label), '[^a-z0-9]+', ' ', 'g')) AS label) norm
               CROSS JOIN LATERAL (
                   SELECT CASE
                       WHEN norm.label ~ '(full time|fulltime|permanent)' THEN 'full_time'
                       WHEN norm.label ~ '(part time|parttime)' THEN 'part_time'
                       WHEN norm.label ~ '(intern|co op)' THEN 'internship'
                       WHEN norm.label LIKE '%freelance%' THEN 'freelance'
                       WHEN norm.label ~ '(b2b|business to business)' THEN 'b2b'
                       WHEN norm.label ~ '(temporary|fixed term|fixedterm)' THEN 'temporary'
                       WHEN norm.label LIKE '%contract%' THEN 'contract'
                       ELSE 'unknown'
                   END AS mapped
                   UNION ALL
                   SELECT 'internship'
                   WHERE norm.label ~ '(full time|fulltime|permanent)'
                     AND norm.label ~ '(intern|co op)'
               ) types
               WHERE trim(norm.label) <> ''
               ORDER BY mapped
           ), '{}'::text[]) AS employment_types
    FROM jobs j
), changed AS (
    UPDATE jobs j
       SET employment_types = n.employment_types,
           updated_at = now()
      FROM normalized n
     WHERE j.id = n.id
       AND j.employment_types IS DISTINCT FROM n.employment_types
    RETURNING j.id, j.company_id
)
INSERT INTO outbox_events (id,event_type,aggregate_type,aggregate_id,payload)
SELECT gen_random_uuid(), 'job.updated', 'job', changed.id::text,
       jsonb_build_object('job_id', changed.id::text, 'company_id', changed.company_id::text,
                          'reason', 'employment_type_canonicalization')
FROM changed;

-- +goose Down
-- The original provider-specific labels cannot be reconstructed safely.
