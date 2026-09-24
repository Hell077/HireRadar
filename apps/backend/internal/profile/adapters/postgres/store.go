package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Get(ctx context.Context, userID user.UserID) (domain.Profile, error) {
	p := domain.Profile{UserID: userID}
	var amount sql.NullInt64
	var currency string
	err := s.pool.QueryRow(ctx, `
		SELECT first_name, last_name, country, city, timezone, experience_years, seniority,
		       desired_salary_amount, desired_salary_currency
		FROM user_profiles WHERE user_id = $1`, string(userID)).Scan(
		&p.FirstName, &p.LastName, &p.Country, &p.City, &p.Timezone, &p.ExperienceYears,
		&p.Seniority, &amount, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return domain.Profile{}, fmt.Errorf("load candidate profile: %w", err)
	}
	if amount.Valid {
		p.DesiredSalary = &domain.Money{Amount: amount.Int64, Currency: currency}
	}
	return p, nil
}

func (s *Store) Save(ctx context.Context, p domain.Profile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin profile update: %w", err)
	}
	defer tx.Rollback(ctx)
	var amount *int64
	currency := ""
	if p.DesiredSalary != nil {
		amount = &p.DesiredSalary.Amount
		currency = p.DesiredSalary.Currency
	}
	var changedUserID string
	err = tx.QueryRow(ctx, `
		INSERT INTO user_profiles (user_id, first_name, last_name, country, city, timezone,
		    experience_years, seniority, desired_salary_amount, desired_salary_currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id) DO UPDATE SET
		    first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name,
		    country = EXCLUDED.country, city = EXCLUDED.city, timezone = EXCLUDED.timezone,
		    experience_years = EXCLUDED.experience_years, seniority = EXCLUDED.seniority,
		    desired_salary_amount = EXCLUDED.desired_salary_amount,
		    desired_salary_currency = EXCLUDED.desired_salary_currency, updated_at = now()
		WHERE ROW(user_profiles.first_name, user_profiles.last_name, user_profiles.country,
		    user_profiles.city, user_profiles.timezone, user_profiles.experience_years,
		    user_profiles.seniority, user_profiles.desired_salary_amount, user_profiles.desired_salary_currency)
		IS DISTINCT FROM ROW(EXCLUDED.first_name, EXCLUDED.last_name, EXCLUDED.country,
		    EXCLUDED.city, EXCLUDED.timezone, EXCLUDED.experience_years, EXCLUDED.seniority,
		    EXCLUDED.desired_salary_amount, EXCLUDED.desired_salary_currency)
		RETURNING user_id`, string(p.UserID), p.FirstName, p.LastName, p.Country, p.City,
		p.Timezone, p.ExperienceYears, p.Seniority, amount, currency).Scan(&changedUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("save candidate profile: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload)
		VALUES ($1, 'profile.changed', 'user', $2, jsonb_build_object('user_id', $2::text))`,
		uuid.NewString(), changedUserID)
	if err != nil {
		return fmt.Errorf("publish profile change: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit candidate profile: %w", err)
	}
	return nil
}
