-- +goose Up
-- Public aggregators expose rolling feeds, which the source worker must not
-- interpret as complete board snapshots.
ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_source_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_source_type_check
    CHECK (source_type IN ('greenhouse','lever','ashby','github','remoteok','jobicy','weworkremotely'));

-- +goose Down
ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_source_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_source_type_check
    CHECK (source_type IN ('greenhouse','lever','ashby','github'));
