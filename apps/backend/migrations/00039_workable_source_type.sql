-- +goose Up
ALTER TABLE sources DROP CONSTRAINT sources_source_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_source_type_check
    CHECK (source_type IN ('greenhouse','lever','ashby','github','remoteok','jobicy','weworkremotely','workable'));

-- Previously detected Workable candidates were held as unsupported. Requeue
-- them for normal provider resolution and public-feed verification.
UPDATE source_candidates
   SET status='discovered',next_check_at=now(),last_error=NULL,updated_at=now()
 WHERE status='unsupported' AND detected_provider='workable';

-- +goose Down
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM sources WHERE source_type='workable') THEN
        RAISE EXCEPTION 'cannot roll back Workable support while Workable sources exist';
    END IF;
END;
$$;
UPDATE source_candidates SET status='unsupported',source_id=NULL
 WHERE status='source_created' AND detected_provider='workable';
ALTER TABLE sources DROP CONSTRAINT sources_source_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_source_type_check
    CHECK (source_type IN ('greenhouse','lever','ashby','github','remoteok','jobicy','weworkremotely'));
