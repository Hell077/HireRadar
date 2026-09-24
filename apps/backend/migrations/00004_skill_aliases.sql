-- +goose Up
INSERT INTO skills (id, name, normalized_name) VALUES
    (md5('skill:go')::uuid, 'Go', 'go'),
    (md5('skill:react')::uuid, 'React', 'react'),
    (md5('skill:postgresql')::uuid, 'PostgreSQL', 'postgresql'),
    (md5('skill:next.js')::uuid, 'Next.js', 'next.js'),
    (md5('skill:kubernetes')::uuid, 'Kubernetes', 'kubernetes'),
    (md5('skill:javascript')::uuid, 'JavaScript', 'javascript'),
    (md5('skill:typescript')::uuid, 'TypeScript', 'typescript')
ON CONFLICT (normalized_name) DO NOTHING;

INSERT INTO skill_aliases (id, skill_id, alias, normalized_alias)
SELECT md5('skill-alias:' || aliases.alias)::uuid, skills.id, aliases.alias, aliases.alias
FROM (VALUES
    ('golang', 'go'), ('go lang', 'go'),
    ('react.js', 'react'), ('reactjs', 'react'),
    ('postgres', 'postgresql'),
    ('next', 'next.js'), ('nextjs', 'next.js'),
    ('k8s', 'kubernetes'),
    ('js', 'javascript'), ('ts', 'typescript')
) AS aliases(alias, canonical)
JOIN skills ON skills.normalized_name = aliases.canonical
ON CONFLICT (normalized_alias) DO NOTHING;

-- +goose Down
-- Preserve catalog rows that may already be referenced by user_skills.
SELECT 1;
