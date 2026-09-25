package postgres

import (
	"context"
	"fmt"
	"slices"
	"strings"

	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
)

func (s *Store) GetPositions(ctx context.Context, userID user.UserID) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT title FROM user_positions WHERE user_id = $1 ORDER BY normalized_title", string(userID))
	if err != nil {
		return nil, fmt.Errorf("load desired positions: %w", err)
	}
	defer rows.Close()
	titles := []string{}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}

func (s *Store) SavePositions(ctx context.Context, userID user.UserID, titles []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin position update: %w", err)
	}
	defer tx.Rollback(ctx)
	var owner string
	if err := tx.QueryRow(ctx, "SELECT id FROM users WHERE id = $1 FOR UPDATE", string(userID)).Scan(&owner); err != nil {
		return fmt.Errorf("lock position owner: %w", err)
	}
	wanted := make([]string, len(titles))
	for i, title := range titles {
		wanted[i] = strings.ToLower(title)
	}
	slices.Sort(wanted)
	current := []string{}
	rows, err := tx.Query(ctx, "SELECT normalized_title FROM user_positions WHERE user_id = $1 ORDER BY normalized_title", string(userID))
	if err != nil {
		return fmt.Errorf("load existing positions: %w", err)
	}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			rows.Close()
			return err
		}
		current = append(current, title)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if slices.Equal(current, wanted) {
		return nil
	}
	if _, err := tx.Exec(ctx, "DELETE FROM user_positions WHERE user_id = $1", string(userID)); err != nil {
		return fmt.Errorf("replace positions: %w", err)
	}
	for _, title := range titles {
		if _, err := tx.Exec(ctx, "INSERT INTO user_positions (id, user_id, title, normalized_title) VALUES ($1, $2, $3, $4)",
			uuid.NewString(), string(userID), title, strings.ToLower(title)); err != nil {
			return fmt.Errorf("insert position: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload)
		VALUES ($1, 'profile.changed', 'user', $2, jsonb_build_object('user_id', $2::text))`, uuid.NewString(), string(userID)); err != nil {
		return fmt.Errorf("publish position change: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit positions: %w", err)
	}
	return nil
}
