-- +goose Up
ALTER TABLE parsed_resumes
    ADD COLUMN experiences jsonb NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(experiences) = 'array');

-- +goose Down
ALTER TABLE parsed_resumes DROP COLUMN experiences;
