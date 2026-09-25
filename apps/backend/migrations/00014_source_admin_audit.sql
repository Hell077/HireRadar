-- +goose Up
CREATE TABLE source_admin_audit (
    id bigserial PRIMARY KEY,
    source_id text NOT NULL,
    actor text NOT NULL,
    action text NOT NULL CHECK (action IN ('settings_updated','sync_requested')),
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX source_admin_audit_created_idx ON source_admin_audit(created_at DESC, id DESC);

-- +goose Down
DROP TABLE source_admin_audit;
