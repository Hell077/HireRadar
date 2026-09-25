package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) GetPreferences(ctx context.Context, userID user.UserID) (domain.Preferences, error) {
	p := domain.Preferences{
		RemotePolicies: []string{}, EmploymentTypes: []string{}, AllowedRegions: []string{}, ExcludedCountries: []string{},
		MaximumJobAgeDays: 30, NotificationsEnabled: true,
	}
	var minimum sql.NullInt64
	var currency string
	err := s.pool.QueryRow(ctx, `SELECT remote_policies, employment_types, allowed_regions, excluded_countries,
		minimum_salary_amount, minimum_salary_currency, minimum_match_score, maximum_job_age_days, notifications_enabled
		FROM job_preferences WHERE user_id=$1`, string(userID)).Scan(&p.RemotePolicies, &p.EmploymentTypes, &p.AllowedRegions, &p.ExcludedCountries, &minimum, &currency, &p.MinimumMatchScore, &p.MaximumJobAgeDays, &p.NotificationsEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return domain.Preferences{}, fmt.Errorf("load job preferences: %w", err)
	}
	if minimum.Valid {
		p.MinimumSalary = &domain.Money{Amount: minimum.Int64, Currency: currency}
	}
	return p, nil
}

func (s *Store) SavePreferences(ctx context.Context, userID user.UserID, p domain.Preferences) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin preferences update: %w", err)
	}
	defer tx.Rollback(ctx)
	var amount *int64
	currency := ""
	if p.MinimumSalary != nil {
		amount = &p.MinimumSalary.Amount
		currency = p.MinimumSalary.Currency
	}
	var changedID string
	err = tx.QueryRow(ctx, `INSERT INTO job_preferences (user_id,remote_policies,employment_types,allowed_regions,excluded_countries,
		minimum_salary_amount,minimum_salary_currency,minimum_match_score,maximum_job_age_days,notifications_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT(user_id) DO UPDATE SET remote_policies=EXCLUDED.remote_policies,employment_types=EXCLUDED.employment_types,
		allowed_regions=EXCLUDED.allowed_regions,excluded_countries=EXCLUDED.excluded_countries,
		minimum_salary_amount=EXCLUDED.minimum_salary_amount,minimum_salary_currency=EXCLUDED.minimum_salary_currency,
		minimum_match_score=EXCLUDED.minimum_match_score,maximum_job_age_days=EXCLUDED.maximum_job_age_days,
		notifications_enabled=EXCLUDED.notifications_enabled,updated_at=now()
		WHERE ROW(job_preferences.remote_policies,job_preferences.employment_types,job_preferences.allowed_regions,
		job_preferences.excluded_countries,job_preferences.minimum_salary_amount,job_preferences.minimum_salary_currency,
		job_preferences.minimum_match_score,job_preferences.maximum_job_age_days,job_preferences.notifications_enabled)
		IS DISTINCT FROM ROW(EXCLUDED.remote_policies,EXCLUDED.employment_types,EXCLUDED.allowed_regions,
		EXCLUDED.excluded_countries,EXCLUDED.minimum_salary_amount,EXCLUDED.minimum_salary_currency,
		EXCLUDED.minimum_match_score,EXCLUDED.maximum_job_age_days,EXCLUDED.notifications_enabled)
		RETURNING user_id`, string(userID), p.RemotePolicies, p.EmploymentTypes, p.AllowedRegions, p.ExcludedCountries, amount, currency, p.MinimumMatchScore, p.MaximumJobAgeDays, p.NotificationsEnabled).Scan(&changedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("save job preferences: %w", err)
	}
	if err := publishProfileChanged(ctx, tx, changedID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit job preferences: %w", err)
	}
	return nil
}

func (s *Store) GetSourcePreferences(ctx context.Context, userID user.UserID) ([]domain.SourcePreference, error) {
	rows, err := s.pool.Query(ctx, "SELECT source_id,enabled FROM user_source_preferences WHERE user_id=$1 ORDER BY source_id", string(userID))
	if err != nil {
		return nil, fmt.Errorf("load source preferences: %w", err)
	}
	defer rows.Close()
	result := []domain.SourcePreference{}
	for rows.Next() {
		var value domain.SourcePreference
		if err := rows.Scan(&value.SourceID, &value.Enabled); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) SaveSourcePreferences(ctx context.Context, userID user.UserID, values []domain.SourcePreference) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin source preference update: %w", err)
	}
	defer tx.Rollback(ctx)
	var owner string
	if err := tx.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", string(userID)).Scan(&owner); err != nil {
		return fmt.Errorf("lock preference owner: %w", err)
	}
	current := []domain.SourcePreference{}
	rows, err := tx.Query(ctx, "SELECT source_id,enabled FROM user_source_preferences WHERE user_id=$1 ORDER BY source_id", string(userID))
	if err != nil {
		return fmt.Errorf("load existing source preferences: %w", err)
	}
	for rows.Next() {
		var item domain.SourcePreference
		if err := rows.Scan(&item.SourceID, &item.Enabled); err != nil {
			rows.Close()
			return err
		}
		current = append(current, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	wanted := slices.Clone(values)
	slices.SortFunc(wanted, func(a, b domain.SourcePreference) int { return strings.Compare(a.SourceID, b.SourceID) })
	if slices.Equal(current, wanted) {
		return nil
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_source_preferences WHERE user_id=$1", string(userID)); err != nil {
		return fmt.Errorf("replace source preferences: %w", err)
	}
	for _, item := range wanted {
		if _, err := tx.Exec(ctx, "INSERT INTO user_source_preferences(user_id,source_id,enabled) VALUES($1,$2,$3)", string(userID), item.SourceID, item.Enabled); err != nil {
			return fmt.Errorf("insert source preference: %w", err)
		}
	}
	if err := publishProfileChanged(ctx, tx, string(userID)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit source preferences: %w", err)
	}
	return nil
}

func publishProfileChanged(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
		VALUES($1,'profile.changed','user',$2,jsonb_build_object('user_id',$2::text))`, uuid.NewString(), userID); err != nil {
		return fmt.Errorf("publish profile change: %w", err)
	}
	return nil
}
