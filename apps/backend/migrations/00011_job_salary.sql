-- +goose Up
ALTER TABLE jobs
    ADD COLUMN salary_min double precision,
    ADD COLUMN salary_max double precision,
    ADD COLUMN salary_currency text,
    ADD COLUMN salary_period text;

ALTER TABLE jobs ADD CONSTRAINT jobs_salary_range_check CHECK (
    (salary_min IS NULL AND salary_max IS NULL AND salary_currency IS NULL AND salary_period IS NULL)
    OR
    (salary_min > 0 AND salary_max >= salary_min AND salary_max <= 100000000
     AND salary_currency IN ('USD','EUR','GBP','CAD','AUD','NZD','SGD','INR','KZT')
     AND salary_period IN ('hour','week','month','year','unspecified'))
);

-- +goose Down
ALTER TABLE jobs DROP CONSTRAINT jobs_salary_range_check;
ALTER TABLE jobs DROP COLUMN salary_min, DROP COLUMN salary_max, DROP COLUMN salary_currency, DROP COLUMN salary_period;
