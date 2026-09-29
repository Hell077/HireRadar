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
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) AccountByTelegramID(ctx context.Context, telegramUserID int64) (user.UserID, domain.NotificationPreferences, bool, error) {
	var userID string
	var enabled bool
	if err := s.pool.QueryRow(ctx, `UPDATE telegram_accounts SET last_interaction_at=now() WHERE telegram_user_id=$1 RETURNING user_id::text,enabled`, telegramUserID).Scan(&userID, &enabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.DefaultPreferences(), false, nil
		}
		return "", domain.NotificationPreferences{}, false, fmt.Errorf("load Telegram command account: %w", err)
	}
	prefs, err := readPreferences(ctx, s.pool, userID)
	if err != nil {
		return "", domain.NotificationPreferences{}, false, fmt.Errorf("load Telegram command preferences: %w", err)
	}
	return user.UserID(userID), prefs, enabled, nil
}

func (s *Store) SetNotificationsEnabled(ctx context.Context, telegramUserID int64, enabled bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram notification setting update: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	if err := tx.QueryRow(ctx, `SELECT user_id::text FROM telegram_accounts WHERE telegram_user_id=$1 AND enabled=true FOR UPDATE`, telegramUserID).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrFeedbackNotOwned
		}
		return fmt.Errorf("verify Telegram account before updating settings: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO notification_preferences(user_id,enabled,minimum_score,immediate,digest_enabled,timezone)
		VALUES($1,$2,70,true,false,'UTC') ON CONFLICT(user_id) DO UPDATE SET enabled=EXCLUDED.enabled,updated_at=now()`, userID, enabled)
	if err != nil {
		return fmt.Errorf("save Telegram notification setting: %w", err)
	}
	if !enabled {
		_, err = tx.Exec(ctx, `UPDATE notifications SET status='cancelled',locked_by=NULL,locked_until=NULL WHERE user_id=$1 AND status IN ('pending','delivering')`, userID)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload)
				SELECT gen_random_uuid(),'match.created','match',m.job_id::text,jsonb_build_object('user_id',m.user_id::text,'job_id',m.job_id::text,'score',m.score::int)
				FROM user_job_matches m JOIN jobs j ON j.id=m.job_id WHERE m.user_id=$1 AND j.status='active'
				AND m.score >= (SELECT minimum_score FROM notification_preferences WHERE user_id=$1)`, userID)
	}
	if err != nil {
		return fmt.Errorf("reschedule Telegram notifications: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram notification setting: %w", err)
	}
	return nil
}

func (s *Store) RecentMatchesForTelegram(ctx context.Context, telegramUserID int64, limit int) ([]application.RecommendedJob, error) {
	if limit < 1 || limit > 10 {
		limit = 5
	}
	rows, err := s.pool.Query(ctx, `SELECT m.job_id::text,j.title,c.name,j.location,j.remote_policy,j.employment_types,j.apply_url,m.score,
		COALESCE(array_agg(DISTINCT sk.name) FILTER (WHERE sk.name IS NOT NULL),'{}'::text[])
		FROM telegram_accounts a JOIN user_job_matches m ON m.user_id=a.user_id JOIN jobs j ON j.id=m.job_id JOIN companies c ON c.id=j.company_id
		LEFT JOIN user_profiles p ON p.user_id=m.user_id LEFT JOIN job_skills js ON js.job_id=j.id LEFT JOIN skills sk ON sk.id=js.skill_id
		WHERE a.telegram_user_id=$1 AND a.enabled=true AND j.status='active'
		AND m.candidate_version=COALESCE(p.match_version,0) AND m.job_version=j.match_version
		AND NOT EXISTS(SELECT 1 FROM user_job_feedback f WHERE f.user_id=m.user_id AND f.job_id=m.job_id AND f.feedback_type IN ('hidden','not_interested','applied'))
		GROUP BY m.job_id,j.title,c.name,j.location,j.remote_policy,j.employment_types,j.apply_url,m.score,m.computed_at
		ORDER BY m.score DESC,m.computed_at DESC LIMIT $2`, telegramUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Telegram recommended jobs: %w", err)
	}
	defer rows.Close()
	items := make([]application.RecommendedJob, 0, limit)
	for rows.Next() {
		var item application.RecommendedJob
		if err := rows.Scan(&item.JobID, &item.Title, &item.Company, &item.Location, &item.RemotePolicy, &item.Employment, &item.ApplyURL, &item.Score, &item.Skills); err != nil {
			return nil, fmt.Errorf("read Telegram recommended job: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Telegram recommended jobs: %w", err)
	}
	return items, nil
}

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
	err := s.pool.QueryRow(ctx, `SELECT id::text,telegram_user_id,chat_id,username,connected_at,enabled,last_interaction_at FROM telegram_accounts WHERE user_id=$1`, string(userID)).Scan(&account.ID, &account.TelegramUserID, &account.ChatID, &account.Username, &account.ConnectedAt, &account.Enabled, &account.LastInteractionAt)
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
	_, err = tx.Exec(ctx, `INSERT INTO telegram_accounts(id,user_id,telegram_user_id,chat_id,username,enabled,last_interaction_at) VALUES($1,$2,$3,$4,$5,true,now())
		ON CONFLICT(user_id) DO UPDATE SET telegram_user_id=EXCLUDED.telegram_user_id,chat_id=EXCLUDED.chat_id,username=EXCLUDED.username,enabled=true,last_interaction_at=now(),updated_at=now()`, account.ID, userID, account.TelegramUserID, account.ChatID, account.Username)
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

func (s *Store) ListSavedJobs(ctx context.Context, userID user.UserID, limit int) ([]application.SavedJob, error) {
	rows, err := s.pool.Query(ctx, `SELECT j.id::text,j.title,c.name,j.location,j.apply_url,j.status,s.created_at
		FROM saved_jobs s JOIN jobs j ON j.id=s.job_id JOIN companies c ON c.id=j.company_id
		WHERE s.user_id=$1 ORDER BY s.created_at DESC,j.id LIMIT $2`, string(userID), limit)
	if err != nil {
		return nil, fmt.Errorf("list saved jobs: %w", err)
	}
	defer rows.Close()
	items := make([]application.SavedJob, 0)
	for rows.Next() {
		var item application.SavedJob
		if err := rows.Scan(&item.JobID, &item.Title, &item.Company, &item.Location, &item.ApplyURL, &item.Status, &item.SavedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read saved jobs: %w", err)
	}
	return items, nil
}

func (s *Store) RemoveSavedJob(ctx context.Context, userID user.UserID, jobID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM saved_jobs WHERE user_id=$1 AND job_id=$2`, string(userID), jobID)
	if err != nil {
		return fmt.Errorf("remove saved job: %w", err)
	}
	return nil
}

func (s *Store) ApplyAction(ctx context.Context, telegramUserID int64, jobID, action string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin Telegram feedback: %w", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	var chatID int64
	err = tx.QueryRow(ctx, `SELECT user_id::text, chat_id FROM telegram_accounts WHERE telegram_user_id=$1 AND enabled=true FOR UPDATE`, telegramUserID).Scan(&userID, &chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, application.ErrFeedbackNotOwned
	}
	if err != nil {
		return 0, fmt.Errorf("verify Telegram account ownership: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2,0))`, userID, jobID); err != nil {
		return 0, fmt.Errorf("lock Telegram match feedback: %w", err)
	}
	var score, confidence int
	err = tx.QueryRow(ctx, `SELECT score,confidence FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userID, jobID).Scan(&score, &confidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, application.ErrFeedbackNotOwned
	}
	if err != nil {
		return 0, fmt.Errorf("verify Telegram match ownership: %w", err)
	}
	var removed int64
	switch action {
	case "save":
		_, err = tx.Exec(ctx, `INSERT INTO saved_jobs(user_id,job_id) VALUES($1,$2) ON CONFLICT(user_id,job_id) DO NOTHING`, userID, jobID)
	case "hide", "applied", "relevant", "not_relevant":
		feedbackType := "hidden"
		if action == "applied" {
			feedbackType = "applied"
		} else if action == "relevant" {
			feedbackType = "relevant"
		} else if action == "not_relevant" {
			feedbackType = "not_interested"
		}
		_, err = tx.Exec(ctx, `INSERT INTO user_job_feedback(id,user_id,job_id,feedback_type) VALUES($1,$2,$3,$4)
			ON CONFLICT(user_id,job_id,feedback_type) DO UPDATE SET created_at=now()`, uuid.NewString(), userID, jobID, feedbackType)
	default:
		return 0, application.ErrInvalidCallback
	}
	if err != nil {
		return 0, fmt.Errorf("save Telegram feedback: %w", err)
	}
	if action != "save" && action != "relevant" {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND job_id=$2 AND status='pending'`, userID, jobID); err != nil {
			return 0, fmt.Errorf("cancel feedback notification: %w", err)
		}
		tag, err := tx.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userID, jobID)
		if err != nil {
			return 0, fmt.Errorf("remove dismissed match: %w", err)
		}
		removed = tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit Telegram feedback: %w", err)
	}
	if removed > 0 {
		observability.DefaultMetrics.Add("hireradar_matches_removed_total", nil, float64(removed))
	}
	recordFeedbackOutcome(action, score, confidence)
	return chatID, nil
}

func (s *Store) ApplyUserAction(ctx context.Context, userID user.UserID, jobID, action string) error {
	return s.ApplyUserFeedback(ctx, userID, jobID, action, "")
}

func (s *Store) ApplyUserFeedback(ctx context.Context, userID user.UserID, jobID, action, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin user job feedback: %w", err)
	}
	defer tx.Rollback(ctx)
	userIDString := string(userID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2,0))`, userIDString, jobID); err != nil {
		return fmt.Errorf("lock user job feedback: %w", err)
	}
	var score, confidence int
	err = tx.QueryRow(ctx, `SELECT score,confidence FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userIDString, jobID).Scan(&score, &confidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrFeedbackNotOwned
	}
	if err != nil {
		return fmt.Errorf("verify user match ownership: %w", err)
	}
	feedbackType := "hidden"
	if action == "applied" {
		feedbackType = "applied"
	} else if action != "hide" {
		return application.ErrInvalidCallback
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_job_feedback(id,user_id,job_id,feedback_type,reason) VALUES($1,$2,$3,$4,NULLIF($5,''))
		ON CONFLICT(user_id,job_id,feedback_type) DO UPDATE SET reason=COALESCE(EXCLUDED.reason,user_job_feedback.reason),created_at=now()`, uuid.NewString(), userIDString, jobID, feedbackType, reason); err != nil {
		return fmt.Errorf("save user job feedback: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE user_id=$1 AND job_id=$2 AND status='pending'`, userIDString, jobID); err != nil {
		return fmt.Errorf("cancel feedback notification: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, userIDString, jobID)
	if err != nil {
		return fmt.Errorf("remove dismissed match: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user job feedback: %w", err)
	}
	if tag.RowsAffected() > 0 {
		observability.DefaultMetrics.Add("hireradar_matches_removed_total", nil, float64(tag.RowsAffected()))
	}
	recordFeedbackOutcome(action, score, confidence)
	return nil
}

func recordFeedbackOutcome(action string, score, confidence int) {
	observability.DefaultMetrics.Add("hireradar_product_feedback_total", map[string]string{
		"action": action, "score_bucket": observability.Bucket100(score), "confidence_bucket": observability.Bucket100(confidence),
	}, 1)
}

func (s *Store) SaveFeedbackReason(ctx context.Context, telegramUserID int64, jobID, reason string) error {
	result, err := s.pool.Exec(ctx, `UPDATE user_job_feedback f SET reason=$3,created_at=now()
		FROM telegram_accounts a
		WHERE a.telegram_user_id=$1 AND a.enabled=true AND f.user_id=a.user_id AND f.job_id=$2 AND f.feedback_type='hidden'`, telegramUserID, jobID, reason)
	if err != nil {
		return fmt.Errorf("save Telegram feedback reason: %w", err)
	}
	if result.RowsAffected() == 0 {
		return application.ErrFeedbackNotOwned
	}
	return nil
}

func (s *Store) OpenNotification(ctx context.Context, notificationID string) (string, int, int, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("begin notification open tracking: %w", err)
	}
	defer tx.Rollback(ctx)
	var applyURL string
	var score, confidence int
	var openedAt sql.NullTime
	err = tx.QueryRow(ctx, `SELECT j.apply_url,n.match_score,n.match_confidence,n.opened_at
		FROM notifications n JOIN jobs j ON j.id=n.job_id
		WHERE n.id=$1 AND n.status='sent' FOR UPDATE OF n`, notificationID).
		Scan(&applyURL, &score, &confidence, &openedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, 0, false, application.ErrNotificationNotFound
	}
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("load notification open target: %w", err)
	}
	firstOpen := !openedAt.Valid
	if firstOpen {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET opened_at=now() WHERE id=$1`, notificationID); err != nil {
			return "", 0, 0, false, fmt.Errorf("record notification open: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", 0, 0, false, fmt.Errorf("commit notification open tracking: %w", err)
	}
	return applyURL, score, confidence, firstOpen, nil
}
