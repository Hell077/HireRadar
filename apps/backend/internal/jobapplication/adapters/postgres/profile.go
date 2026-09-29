package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileStore struct{ pool *pgxpool.Pool }

func NewProfileStore(pool *pgxpool.Pool) *ProfileStore { return &ProfileStore{pool: pool} }

func (s *ProfileStore) GetProfile(ctx context.Context, userID string) (domain.ApplicationProfile, error) {
	profile := domain.ApplicationProfile{UserID: userID}
	var resumeID, currency sql.NullString
	var amount sql.NullInt64
	var notice sql.NullInt32
	var latitude, longitude sql.NullFloat64
	var workAuthorization, customAnswers []byte
	err := s.pool.QueryRow(ctx, `
		SELECT first_name,last_name,email,phone,country,city,address,latitude,longitude,linkedin_url,github_url,website_url,
		       resume_id::text,expected_salary_amount,expected_salary_currency,notice_period_days,
		       work_authorization,custom_answers
		FROM application_profiles WHERE user_id=$1`, userID).Scan(
		&profile.FirstName, &profile.LastName, &profile.Email, &profile.Phone, &profile.Country,
		&profile.City, &profile.Address, &latitude, &longitude, &profile.LinkedInURL, &profile.GitHubURL, &profile.WebsiteURL, &resumeID,
		&amount, &currency, &notice, &workAuthorization, &customAnswers)
	if errors.Is(err, pgx.ErrNoRows) {
		profile.WorkAuthorization = []domain.WorkAuthorization{}
		profile.CustomAnswers = map[string]string{}
		return profile, nil
	}
	if err != nil {
		return domain.ApplicationProfile{}, fmt.Errorf("load application profile: %w", err)
	}
	if resumeID.Valid {
		profile.ResumeID = resumeID.String
	}
	if latitude.Valid {
		value := latitude.Float64
		profile.Latitude = &value
	}
	if longitude.Valid {
		value := longitude.Float64
		profile.Longitude = &value
	}
	if amount.Valid {
		profile.ExpectedSalary = &domain.ApplicationMoney{Amount: amount.Int64, Currency: currency.String}
	}
	if notice.Valid {
		value := int(notice.Int32)
		profile.NoticePeriodDays = &value
	}
	if err := json.Unmarshal(workAuthorization, &profile.WorkAuthorization); err != nil {
		return domain.ApplicationProfile{}, fmt.Errorf("decode application profile authorization: %w", err)
	}
	if err := json.Unmarshal(customAnswers, &profile.CustomAnswers); err != nil {
		return domain.ApplicationProfile{}, fmt.Errorf("decode application profile answers: %w", err)
	}
	return profile, nil
}

func (s *ProfileStore) SaveProfile(ctx context.Context, profile domain.ApplicationProfile) error {
	var resumeID, currency any
	var amount, notice any
	if profile.ResumeID != "" {
		resumeID = profile.ResumeID
	}
	if profile.ExpectedSalary != nil {
		amount = profile.ExpectedSalary.Amount
		currency = profile.ExpectedSalary.Currency
	}
	if profile.NoticePeriodDays != nil {
		notice = *profile.NoticePeriodDays
	}
	workAuthorization, err := json.Marshal(profile.WorkAuthorization)
	if err != nil {
		return fmt.Errorf("encode application profile authorization: %w", err)
	}
	customAnswers, err := json.Marshal(profile.CustomAnswers)
	if err != nil {
		return fmt.Errorf("encode application profile answers: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO application_profiles (
			user_id,first_name,last_name,email,phone,country,city,address,latitude,longitude,linkedin_url,github_url,website_url,
			resume_id,expected_salary_amount,expected_salary_currency,notice_period_days,work_authorization,custom_answers
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb,$19::jsonb)
		ON CONFLICT (user_id) DO UPDATE SET
			first_name=EXCLUDED.first_name,last_name=EXCLUDED.last_name,email=EXCLUDED.email,
			phone=EXCLUDED.phone,country=EXCLUDED.country,city=EXCLUDED.city,address=EXCLUDED.address,
			latitude=EXCLUDED.latitude,longitude=EXCLUDED.longitude,
			linkedin_url=EXCLUDED.linkedin_url,github_url=EXCLUDED.github_url,website_url=EXCLUDED.website_url,
			resume_id=EXCLUDED.resume_id,expected_salary_amount=EXCLUDED.expected_salary_amount,
			expected_salary_currency=EXCLUDED.expected_salary_currency,notice_period_days=EXCLUDED.notice_period_days,
			work_authorization=EXCLUDED.work_authorization,custom_answers=EXCLUDED.custom_answers,updated_at=now()`,
		profile.UserID, profile.FirstName, profile.LastName, profile.Email, profile.Phone,
		profile.Country, profile.City, profile.Address, profile.Latitude, profile.Longitude,
		profile.LinkedInURL, profile.GitHubURL, profile.WebsiteURL,
		resumeID, amount, currency, notice, workAuthorization, customAnswers)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "application_profiles_resume_id_user_id_fkey" {
			return domain.ErrInvalidApplicationProfile
		}
		return fmt.Errorf("save application profile: %w", err)
	}
	return nil
}
