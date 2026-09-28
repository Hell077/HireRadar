-- +goose Up
ALTER TABLE discovery_sources DROP CONSTRAINT discovery_sources_parser_key_check;
ALTER TABLE discovery_sources ADD CONSTRAINT discovery_sources_parser_key_check
    CHECK (parser_key IN ('remoteintech','established_remote','global_hiring','awesome_remote_job','european_remote','remote_by_default','remote_freelancer','remote_developer_directory','github_issue_boards'));

-- +goose Down
DELETE FROM discovery_sources WHERE parser_key IN ('awesome_remote_job','european_remote','remote_by_default','remote_freelancer','remote_developer_directory','github_issue_boards');
ALTER TABLE discovery_sources DROP CONSTRAINT discovery_sources_parser_key_check;
ALTER TABLE discovery_sources ADD CONSTRAINT discovery_sources_parser_key_check
    CHECK (parser_key IN ('remoteintech','established_remote','global_hiring'));
