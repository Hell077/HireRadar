-- +goose Up
ALTER TABLE application_profiles
    ADD COLUMN address text NOT NULL DEFAULT '',
    ADD COLUMN latitude double precision,
    ADD COLUMN longitude double precision,
    ADD CONSTRAINT application_profiles_address_length CHECK (length(address) <= 255),
    ADD CONSTRAINT application_profiles_location_pair CHECK ((latitude IS NULL) = (longitude IS NULL)),
    ADD CONSTRAINT application_profiles_latitude_range CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    ADD CONSTRAINT application_profiles_longitude_range CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180);

-- +goose Down
ALTER TABLE application_profiles
    DROP CONSTRAINT application_profiles_longitude_range,
    DROP CONSTRAINT application_profiles_latitude_range,
    DROP CONSTRAINT application_profiles_location_pair,
    DROP CONSTRAINT application_profiles_address_length,
    DROP COLUMN longitude,
    DROP COLUMN latitude,
    DROP COLUMN address;
