-- +goose Up
ALTER TABLE user_job_feedback
    DROP CONSTRAINT user_job_feedback_feedback_type_check,
    ADD CONSTRAINT user_job_feedback_feedback_type_check
        CHECK (feedback_type IN ('hidden','not_interested','applied','relevant'));

-- +goose Down
DELETE FROM user_job_feedback WHERE feedback_type='relevant';
ALTER TABLE user_job_feedback
    DROP CONSTRAINT user_job_feedback_feedback_type_check,
    ADD CONSTRAINT user_job_feedback_feedback_type_check
        CHECK (feedback_type IN ('hidden','not_interested','applied'));
