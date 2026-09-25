package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxDeliveryAttempts = 12

type Worker struct {
	pool *pgxpool.Pool
	bot  application.Bot
	now  func() time.Time
}

func NewWorker(pool *pgxpool.Pool, bot application.Bot, now func() time.Time) *Worker {
	return &Worker{pool: pool, bot: bot, now: now}
}

// ProcessNext schedules one match event first, then sends one due notification.
func (w *Worker) ProcessNext(ctx context.Context) (bool, error) {
	foundEvent, err := w.processMatchEvent(ctx, "")
	if err != nil {
		return foundEvent, err
	}
	foundDelivery, err := w.deliverNext(ctx)
	return foundEvent || foundDelivery, err
}

func (w *Worker) ProcessEvent(ctx context.Context, eventID string) (bool, error) {
	return w.processMatchEvent(ctx, eventID)
}

func (w *Worker) processMatchEvent(ctx context.Context, onlyEvent string) (bool, error) {
	var eventFilter any
	if onlyEvent != "" {
		eventFilter = onlyEvent
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin match notification event: %w", err)
	}
	defer tx.Rollback(ctx)
	var eventID string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT id::text,payload FROM outbox_events WHERE event_type='match.created' AND processed_at IS NULL AND available_at<=now()
		AND ($1::uuid IS NULL OR id=$1::uuid)
		ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, eventFilter).Scan(&eventID, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim match notification event: %w", err)
	}
	var event struct {
		UserID string `json:"user_id"`
		JobID  string `json:"job_id"`
		Score  int    `json:"score"`
	}
	if err := json.Unmarshal(payload, &event); err != nil || uuid.Validate(event.UserID) != nil || uuid.Validate(event.JobID) != nil || event.Score < 0 || event.Score > 100 {
		if _, updateErr := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE id=$1`, eventID); updateErr != nil {
			return true, fmt.Errorf("discard malformed match notification event: %w", updateErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return true, fmt.Errorf("commit malformed match event: %w", commitErr)
		}
		return true, fmt.Errorf("match notification event %s has invalid payload", eventID)
	}
	preferences, err := readPreferences(ctx, tx, event.UserID)
	if err != nil {
		return w.retryEvent(ctx, tx, eventID, err)
	}
	var connected bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM telegram_accounts WHERE user_id=$1)`, event.UserID).Scan(&connected); err != nil {
		return w.retryEvent(ctx, tx, eventID, err)
	}
	var dismissed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_job_feedback WHERE user_id=$1 AND job_id=$2 AND feedback_type IN ('hidden','not_interested','applied'))`, event.UserID, event.JobID).Scan(&dismissed); err != nil {
		return w.retryEvent(ctx, tx, eventID, err)
	}
	if preferences.Enabled && event.Score >= preferences.MinimumScore && connected && !dismissed {
		now := w.now().UTC()
		scheduled := preferences.ScheduleAt(now)
		if !preferences.Immediate && preferences.DigestEnabled {
			scheduled = nextDigestAt(now, preferences.Timezone)
		}
		immediate := !scheduled.After(now)
		if _, err := tx.Exec(ctx, `INSERT INTO notifications(id,user_id,job_id,scheduled_at)
			VALUES($1,$2,$3,CASE WHEN $5::bool THEN now() ELSE $4 END) ON CONFLICT(user_id,job_id,notification_type) DO UPDATE SET status='pending',scheduled_at=EXCLUDED.scheduled_at,attempts=0,last_error=NULL
			WHERE notifications.status='cancelled'`, uuid.NewString(), event.UserID, event.JobID, scheduled, immediate); err != nil {
			return w.retryEvent(ctx, tx, eventID, err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET processed_at=now() WHERE id=$1`, eventID); err != nil {
		return w.retryEvent(ctx, tx, eventID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit match notification event: %w", err)
	}
	return true, nil
}

func (w *Worker) retryEvent(ctx context.Context, tx pgx.Tx, eventID string, cause error) (bool, error) {
	if _, err := tx.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,available_at=now()+LEAST(3600,POWER(2,LEAST(attempts+1,12))::int)*interval '1 second' WHERE id=$1`, eventID); err != nil {
		return true, fmt.Errorf("schedule match notification retry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit match notification retry: %w", err)
	}
	return true, fmt.Errorf("schedule match notification: %w", cause)
}

type dueNotification struct {
	ID, UserID, JobID string
	ChatID            int64
	Score             int
	Title, Company    string
	Location          string
	ApplyURL          string
	SalaryMinimum     *float64
	SalaryMaximum     *float64
	SalaryCurrency    *string
	SalaryPeriod      *string
	EmploymentTypes   []string
	Skills            string
}

func (w *Worker) deliverNext(ctx context.Context) (bool, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin Telegram delivery: %w", err)
	}
	defer tx.Rollback(ctx)
	var item dueNotification
	err = tx.QueryRow(ctx, `SELECT n.id::text,n.user_id::text,n.job_id::text,a.chat_id,m.score,j.title,c.name,COALESCE(j.location,''),j.apply_url,
		j.salary_min,j.salary_max,j.salary_currency,j.salary_period,j.employment_types,
		COALESCE((SELECT string_agg(sk.name,' · ' ORDER BY sk.normalized_name) FROM job_skills js JOIN skills sk ON sk.id=js.skill_id WHERE js.job_id=j.id),'')
		FROM notifications n JOIN telegram_accounts a ON a.user_id=n.user_id
		JOIN user_job_matches m ON m.user_id=n.user_id AND m.job_id=n.job_id
		JOIN jobs j ON j.id=n.job_id JOIN companies c ON c.id=j.company_id
		WHERE n.status='pending' AND n.scheduled_at<=now() AND j.status='active'
		ORDER BY n.scheduled_at,n.id FOR UPDATE OF n SKIP LOCKED LIMIT 1`).Scan(&item.ID, &item.UserID, &item.JobID, &item.ChatID, &item.Score, &item.Title, &item.Company, &item.Location, &item.ApplyURL, &item.SalaryMinimum, &item.SalaryMaximum, &item.SalaryCurrency, &item.SalaryPeriod, &item.EmploymentTypes, &item.Skills)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim due Telegram notification: %w", err)
	}
	preferences, err := readPreferences(ctx, tx, item.UserID)
	if err != nil {
		return w.retryDelivery(ctx, tx, item.ID, 0, err)
	}
	if !preferences.Enabled || item.Score < preferences.MinimumScore {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET status='cancelled' WHERE id=$1`, item.ID); err != nil {
			return w.retryDelivery(ctx, tx, item.ID, 0, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return true, fmt.Errorf("commit cancelled notification: %w", err)
		}
		return true, nil
	}
	now := w.now().UTC()
	if scheduled := preferences.ScheduleAt(now); scheduled.After(now) {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET scheduled_at=$2 WHERE id=$1`, item.ID, scheduled); err != nil {
			return w.retryDelivery(ctx, tx, item.ID, 0, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return true, fmt.Errorf("commit quiet-hour schedule: %w", err)
		}
		return true, nil
	}
	if preferences.MaxPerDay != nil {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('notification:' || $1,0))`, item.UserID); err != nil {
			return w.retryDelivery(ctx, tx, item.ID, 0, err)
		}
		location, err := time.LoadLocation(preferences.Timezone)
		if err != nil {
			return w.retryDelivery(ctx, tx, item.ID, 0, err)
		}
		local := now.In(location)
		dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).UTC()
		var sent int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND status='sent' AND sent_at >= $2`, item.UserID, dayStart).Scan(&sent); err != nil {
			return w.retryDelivery(ctx, tx, item.ID, 0, err)
		}
		if sent >= *preferences.MaxPerDay {
			nextDay := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location).UTC()
			if _, err := tx.Exec(ctx, `UPDATE notifications SET scheduled_at=$2 WHERE id=$1`, item.ID, nextDay); err != nil {
				return w.retryDelivery(ctx, tx, item.ID, 0, err)
			}
			if err := tx.Commit(ctx); err != nil {
				return true, fmt.Errorf("commit daily limit schedule: %w", err)
			}
			return true, nil
		}
	}
	if w.bot == nil {
		return w.retryDelivery(ctx, tx, item.ID, 0, errors.New("Telegram sender is unavailable"))
	}
	lines := []string{fmt.Sprintf("<b>%d%% match</b>", item.Score), fmt.Sprintf("<b>%s</b> · %s", html.EscapeString(item.Title), html.EscapeString(item.Company))}
	if item.Location != "" {
		lines = append(lines, html.EscapeString(item.Location))
	}
	if item.SalaryMinimum != nil && item.SalaryMaximum != nil && item.SalaryCurrency != nil && item.SalaryPeriod != nil {
		lines = append(lines, fmt.Sprintf("%s %.0f–%.0f / %s", html.EscapeString(*item.SalaryCurrency), *item.SalaryMinimum, *item.SalaryMaximum, html.EscapeString(*item.SalaryPeriod)))
	}
	if len(item.EmploymentTypes) > 0 {
		lines = append(lines, html.EscapeString(strings.Join(item.EmploymentTypes, " · ")))
	}
	if item.Skills != "" {
		lines = append(lines, html.EscapeString(item.Skills))
	}
	text := strings.Join(lines, "\n")
	buttons := [][]application.Button{
		{{Text: "Open", URL: item.ApplyURL}},
		{{Text: "Save", CallbackData: "job:save:" + item.JobID}, {Text: "Hide", CallbackData: "job:hide:" + item.JobID}, {Text: "Applied", CallbackData: "job:applied:" + item.JobID}},
	}
	if err := w.bot.SendMessage(ctx, item.ChatID, text, buttons); err != nil {
		var attempts int
		if scanErr := tx.QueryRow(ctx, `SELECT attempts FROM notifications WHERE id=$1`, item.ID).Scan(&attempts); scanErr != nil {
			return true, fmt.Errorf("read notification retry count after Telegram failure: %w", scanErr)
		}
		return w.retryDelivery(ctx, tx, item.ID, attempts, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notifications SET status='sent',sent_at=now(),attempts=attempts+1,last_error=NULL WHERE id=$1`, item.ID); err != nil {
		return true, fmt.Errorf("mark Telegram notification sent: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit Telegram delivery: %w", err)
	}
	return true, nil
}

func (w *Worker) retryDelivery(ctx context.Context, tx pgx.Tx, id string, attempts int, cause error) (bool, error) {
	message := cause.Error()
	if len(message) > 512 {
		message = message[:512]
	}
	nextAttempt := attempts + 1
	if nextAttempt >= maxDeliveryAttempts {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET status='failed',attempts=$2,last_error=$3 WHERE id=$1`, id, nextAttempt, message); err != nil {
			return true, fmt.Errorf("dead-letter Telegram notification: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET attempts=$2,last_error=$3,scheduled_at=now()+LEAST(3600,POWER(2,LEAST($2,12))::int)*interval '1 second' WHERE id=$1`, id, nextAttempt, message); err != nil {
			return true, fmt.Errorf("schedule Telegram retry: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit Telegram retry: %w", err)
	}
	return true, fmt.Errorf("deliver Telegram notification: %w", cause)
}

func readPreferences(ctx context.Context, tx pgx.Tx, userID string) (domain.NotificationPreferences, error) {
	prefs := domain.DefaultPreferences()
	var start, end sql.NullString
	var maxPerDay sql.NullInt64
	err := tx.QueryRow(ctx, `SELECT enabled,minimum_score,immediate,digest_enabled,timezone,quiet_start::text,quiet_end::text,max_per_day FROM notification_preferences WHERE user_id=$1`, userID).Scan(&prefs.Enabled, &prefs.MinimumScore, &prefs.Immediate, &prefs.DigestEnabled, &prefs.Timezone, &start, &end, &maxPerDay)
	if errors.Is(err, pgx.ErrNoRows) {
		return prefs, nil
	}
	if err != nil {
		return domain.NotificationPreferences{}, err
	}
	if start.Valid {
		value := strings.TrimSpace(start.String)[:5]
		prefs.QuietStart = &value
	}
	if end.Valid {
		value := strings.TrimSpace(end.String)[:5]
		prefs.QuietEnd = &value
	}
	if maxPerDay.Valid {
		value := int(maxPerDay.Int64)
		prefs.MaxPerDay = &value
	}
	return prefs, nil
}

func nextDigestAt(now time.Time, zone string) time.Time {
	location, err := time.LoadLocation(zone)
	if err != nil {
		return now.Add(24 * time.Hour)
	}
	local := now.In(location)
	result := time.Date(local.Year(), local.Month(), local.Day(), 9, 0, 0, 0, location)
	if !result.After(local) {
		result = result.AddDate(0, 0, 1)
	}
	return result.UTC()
}
