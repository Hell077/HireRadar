package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/Hell077/HireRadar/apps/backend/internal/profile/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type skillRow struct {
	ID    string
	Name  string
	Years *float64
	Level string
}

func (s *Store) GetSkills(ctx context.Context, userID user.UserID) ([]domain.Skill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.name, us.years::float8, us.level FROM user_skills AS us
		JOIN skills AS s ON s.id = us.skill_id WHERE us.user_id = $1 ORDER BY s.normalized_name`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("load candidate skills: %w", err)
	}
	defer rows.Close()
	result := []domain.Skill{}
	for rows.Next() {
		var skill domain.Skill
		if err := rows.Scan(&skill.Name, &skill.Years, &skill.Level); err != nil {
			return nil, err
		}
		result = append(result, skill)
	}
	return result, rows.Err()
}

func (s *Store) SaveSkills(ctx context.Context, userID user.UserID, skills []domain.Skill) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin skill update: %w", err)
	}
	defer tx.Rollback(ctx)
	var owner string
	if err := tx.QueryRow(ctx, "SELECT id FROM users WHERE id = $1 FOR UPDATE", string(userID)).Scan(&owner); err != nil {
		return fmt.Errorf("lock skill owner: %w", err)
	}
	wanted := make([]skillRow, 0, len(skills))
	seen := map[string]bool{}
	for _, skill := range skills {
		key := strings.ToLower(skill.Name)
		var row skillRow
		err = tx.QueryRow(ctx, `
			SELECT id::text, name FROM skills WHERE normalized_name = $1
			UNION ALL
			SELECT s.id::text, s.name FROM skill_aliases a JOIN skills s ON s.id = a.skill_id
			WHERE a.normalized_alias = $1 LIMIT 1`, key).Scan(&row.ID, &row.Name)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				INSERT INTO skills (id, name, normalized_name) VALUES ($1, $2, $3)
				ON CONFLICT (normalized_name) DO UPDATE SET name = skills.name
				RETURNING id::text, name`, uuid.NewString(), skill.Name, key).Scan(&row.ID, &row.Name)
		}
		if err != nil {
			return fmt.Errorf("resolve skill: %w", err)
		}
		if seen[row.ID] {
			return domain.ErrInvalidSkills
		}
		seen[row.ID] = true
		row.Years, row.Level = skill.Years, skill.Level
		wanted = append(wanted, row)
	}
	current := []skillRow{}
	rows, err := tx.Query(ctx, `SELECT us.skill_id::text, s.name, us.years::float8, us.level
		FROM user_skills us JOIN skills s ON s.id = us.skill_id WHERE us.user_id = $1`, string(userID))
	if err != nil {
		return fmt.Errorf("load existing skills: %w", err)
	}
	for rows.Next() {
		var row skillRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Years, &row.Level); err != nil {
			rows.Close()
			return err
		}
		current = append(current, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	sortRows := func(items []skillRow) {
		slices.SortFunc(items, func(a, b skillRow) int { return strings.Compare(a.ID, b.ID) })
	}
	sortRows(current)
	sortRows(wanted)
	if reflect.DeepEqual(current, wanted) {
		return nil
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_skills WHERE user_id = $1", string(userID)); err != nil {
		return fmt.Errorf("replace candidate skills: %w", err)
	}
	for _, row := range wanted {
		if _, err := tx.Exec(ctx, `INSERT INTO user_skills (user_id, skill_id, years, level, source, confirmed)
			VALUES ($1, $2, $3, $4, 'manual', true)`, string(userID), row.ID, row.Years, row.Level); err != nil {
			return fmt.Errorf("insert candidate skill: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload)
		VALUES ($1, 'profile.changed', 'user', $2, jsonb_build_object('user_id', $2::text))`, uuid.NewString(), string(userID)); err != nil {
		return fmt.Errorf("publish skill change: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit candidate skills: %w", err)
	}
	return nil
}
