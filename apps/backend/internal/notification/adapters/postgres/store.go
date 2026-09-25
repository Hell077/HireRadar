package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) CreateLink(ctx context.Context, userID user.UserID, id string, tokenHash []byte, expiresAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram link: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE telegram_link_tokens SET used_at=now() WHERE user_id=$1 AND used_at IS NULL`, string(userID)); err != nil {
		return fmt.Errorf("expire prior Telegram links: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO telegram_link_tokens(id,user_id,token_hash,expires_at) VALUES($1,$2,$3,$4)`, id, string(userID), tokenHash, expiresAt); err != nil {
		return fmt.Errorf("store Telegram link: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram link: %w", err)
	}
	return nil
}

func (s *Store) Connection(ctx context.Context, userID user.UserID) (application.Account, error) {
	var account application.Account
	err := s.pool.QueryRow(ctx, `SELECT id::text,telegram_user_id,chat_id,username,connected_at FROM telegram_accounts WHERE user_id=$1`, string(userID)).Scan(&account.ID, &account.TelegramUserID, &account.ChatID, &account.Username, &account.ConnectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Account{}, nil
	}
	if err != nil {
		return application.Account{}, fmt.Errorf("load Telegram connection: %w", err)
	}
	return account, nil
}

func (s *Store) ConsumeLink(ctx context.Context, tokenHash []byte, account application.Account) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram link confirmation: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `UPDATE telegram_link_tokens SET used_at=now() WHERE token_hash=$1 AND used_at IS NULL AND expires_at>now() RETURNING user_id::text`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrLinkExpired
	}
	if err != nil {
		return fmt.Errorf("consume Telegram link token: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO telegram_accounts(id,user_id,telegram_user_id,chat_id,username) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(user_id) DO UPDATE SET telegram_user_id=EXCLUDED.telegram_user_id,chat_id=EXCLUDED.chat_id,username=EXCLUDED.username,updated_at=now()`, account.ID, userID, account.TelegramUserID, account.ChatID, account.Username)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "telegram_user_id") {
			return application.ErrTelegramInUse
		}
		return fmt.Errorf("save Telegram account: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
		SELECT gen_random_uuid(),'match.created','match',m.job_id::text,jsonb_build_object('user_id',m.user_id::text,'job_id',m.job_id::text,'score',m.score::int)
		FROM user_job_matches m WHERE m.user_id=$1`, userID); err != nil {
		return fmt.Errorf("queue existing matches for Telegram: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram link confirmation: %w", err)
	}
	return nil
}

func (s *Store) Disconnect(ctx context.Context, userID user.UserID) error {
	_, err := s.pool.Exec(ctx, `WITH removed AS (DELETE FROM telegram_accounts WHERE user_id=$1)
		UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND status='pending'`, string(userID))
	if err != nil {
		return fmt.Errorf("disconnect Telegram account: %w", err)
	}
	return nil
}

func (s *Store) GetPreferences(ctx context.Context, userID user.UserID) (domain.NotificationPreferences, error) {
	preferences := domain.DefaultPreferences()
	var start, end sql.NullString
	var maxPerDay sql.NullInt64
	err := s.pool.QueryRow(ctx, `SELECT enabled,minimum_score,immediate,digest_enabled,timezone,quiet_start::text,quiet_end::text,max_per_day
		FROM notification_preferences WHERE user_id=$1`, string(userID)).Scan(&preferences.Enabled, &preferences.MinimumScore, &preferences.Immediate, &preferences.DigestEnabled, &preferences.Timezone, &start, &end, &maxPerDay)
	if errors.Is(err, pgx.ErrNoRows) {
		return preferences, nil
	}
	if err != nil {
		return domain.NotificationPreferences{}, fmt.Errorf("load notification preferences: %w", err)
	}
	if start.Valid {
		value := start.String[:5]
		preferences.QuietStart = &value
	}
	if end.Valid {
		value := end.String[:5]
		preferences.QuietEnd = &value
	}
	if maxPerDay.Valid {
		value := int(maxPerDay.Int64)
		preferences.MaxPerDay = &value
	}
	return preferences, nil
}

func (s *Store) SavePreferences(ctx context.Context, userID user.UserID, preferences domain.NotificationPreferences) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin notification preference update: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO notification_preferences(user_id,enabled,minimum_score,immediate,digest_enabled,timezone,quiet_start,quiet_end,max_per_day)
		VALUES($1,$2,$3,$4,$5,$6,$7::time,$8::time,$9)
		ON CONFLICT(user_id) DO UPDATE SET enabled=EXCLUDED.enabled,minimum_score=EXCLUDED.minimum_score,immediate=EXCLUDED.immediate,
		digest_enabled=EXCLUDED.digest_enabled,timezone=EXCLUDED.timezone,quiet_start=EXCLUDED.quiet_start,quiet_end=EXCLUDED.quiet_end,max_per_day=EXCLUDED.max_per_day,updated_at=now()`,
		string(userID), preferences.Enabled, preferences.MinimumScore, preferences.Immediate, preferences.DigestEnabled, preferences.Timezone, preferences.QuietStart, preferences.QuietEnd, preferences.MaxPerDay)
	if err != nil {
		return fmt.Errorf("save notification preferences: %w", err)
	}
	if !preferences.Enabled {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND status='pending'`, string(userID)); err != nil {
			return fmt.Errorf("cancel disabled notifications: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET scheduled_at=now() WHERE user_id=$1 AND status='pending'`, string(userID)); err != nil {
			return fmt.Errorf("reschedule pending notifications: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
			SELECT gen_random_uuid(),'match.created','match',m.job_id::text,jsonb_build_object('user_id',m.user_id::text,'job_id',m.job_id::text,'score',m.score::int)
			FROM user_job_matches m JOIN jobs j ON j.id=m.job_id WHERE m.user_id=$1 AND j.status='active'`, string(userID)); err != nil {
			return fmt.Errorf("queue current matches after preference update: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit notification preferences: %w", err)
	}
	return nil
}

func (s *Store) ApplyAction(ctx context.Context, telegramUserID int64, jobID, action string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram feedback: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM telegram_accounts WHERE telegram_user_id=$1 FOR UPDATE`, telegramUserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrFeedbackNotOwned
	}
	if err != nil {
		return fmt.Errorf("verify Telegram account ownership: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2,0))`, userID, jobID); err != nil {
		return fmt.Errorf("lock Telegram match feedback: %w", err)
	}
	var ownsMatch bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_job_matches WHERE user_id=$1 AND job_id=$2)`, userID, jobID).Scan(&ownsMatch); err != nil {
		return fmt.Errorf("verify Telegram match ownership: %w", err)
	}
	if !ownsMatch {
		return application.ErrFeedbackNotOwned
	}
	switch action {
	case "save":
		_, err = tx.Exec(ctx, `INSERT INTO saved_jobs(user_id,job_id) VALUES($1,$2) ON CONFLICT(user_id,job_id) DO NOTHING`, userID, jobID)
	case "hide", "applied":
		feedbackType := "hidden"
		if action == "applied" {
			feedbackType = "applied"
		}
		_, err = tx.Exec(ctx, `INSERT INTO user_job_feedback(id,user_id,job_id,feedback_type) VALUES($1,$2,$3,$4)
			ON CONFLICT(user_id,job_id,feedback_type) DO UPDATE SET created_at=now()`, uuid.NewString(), userID, jobID, feedbackType)
	default:
		return application.ErrInvalidCallback
	}
	if err != nil {
		return fmt.Errorf("save Telegram feedback: %w", err)
	}
	if action != "save" {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND job_id=$2 AND status='pending'`, userID, jobID); err != nil {
			return fmt.Errorf("cancel feedback notification: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userID, jobID); err != nil {
			return fmt.Errorf("remove dismissed match: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram feedback: %w", err)
	}
	return nil
}
