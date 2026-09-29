package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	jobapplicationdomain "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/observability"
	"github.com/Hell077/HireRadar/apps/backend/internal/outbox"
	outboxpostgres "github.com/Hell077/HireRadar/apps/backend/internal/outbox/adapters/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxDeliveryAttempts = 12
const notificationLease = 5 * time.Minute

type Worker struct {
	pool        *pgxpool.Pool
	bot         application.Bot
	now         func() time.Time
	outbox      outbox.Store
	workerID    string
	openURLBase string
}

func NewWorker(pool *pgxpool.Pool, bot application.Bot, now func() time.Time, openURLBase ...string) *Worker {
	base := ""
	if len(openURLBase) > 0 {
		if parsed, err := url.ParseRequestURI(strings.TrimSpace(openURLBase[0])); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil {
			base = strings.TrimRight(openURLBase[0], "/")
		}
	}
	return &Worker{pool: pool, bot: bot, now: now, outbox: outboxpostgres.NewStore(pool), workerID: uuid.NewString(), openURLBase: base}
}

// ProcessNext schedules one match event first, then sends one due notification.
func (w *Worker) ProcessNext(ctx context.Context) (bool, error) {
	w.refreshQueueMetrics(ctx)
	foundApplicationEvent, err := w.processApplicationEvent(ctx, "")
	if err != nil || foundApplicationEvent {
		return foundApplicationEvent, err
	}
	foundEvent, err := w.processMatchEvent(ctx, "")
	if err != nil {
		return foundEvent, err
	}
	foundDelivery, err := w.deliverNext(ctx)
	return foundEvent || foundDelivery, err
}

func (w *Worker) refreshQueueMetrics(ctx context.Context) {
	metrics := observability.DefaultMetrics
	if !metrics.ShouldRefresh("notification_queue", time.Now(), 30*time.Second) {
		return
	}
	var depth int64
	var oldestAge float64
	err := w.pool.QueryRow(ctx, `SELECT COALESCE(sum(queue_depth),0),COALESCE(max(oldest_age),0) FROM (
		SELECT count(*) AS queue_depth,COALESCE(GREATEST(0,EXTRACT(EPOCH FROM now()-MIN(scheduled_at) FILTER (WHERE status='pending'))),0) AS oldest_age
		FROM notifications WHERE status IN ('pending','delivering')
		UNION ALL
		SELECT count(*) AS queue_depth,COALESCE(GREATEST(0,EXTRACT(EPOCH FROM now()-MIN(occurred_at) FILTER (WHERE status='pending'))),0) AS oldest_age
		FROM outbox_events WHERE (event_type='match.created' OR event_type LIKE 'application.status.%')
		AND status IN ('pending','processing')
	) q`).Scan(&depth, &oldestAge)
	if err == nil {
		metrics.SetGauge("hireradar_notification_queue_depth", nil, float64(depth))
		metrics.SetGauge("hireradar_notification_queue_oldest_pending_age_seconds", nil, oldestAge)
	}
}

func (w *Worker) processApplicationEvent(ctx context.Context, onlyEvent string) (found bool, retErr error) {
	started := time.Now()
	eventType := "unknown"
	defer func() {
		if found {
			observability.DefaultMetrics.Add("hireradar_notification_status_events_total", map[string]string{"event_type": eventType}, 1)
			observability.DefaultMetrics.ObserveDuration("hireradar_notification_status_processing_duration", nil, time.Since(started))
		}
	}()
	claimed, err := w.outbox.Claim(ctx, w.workerID, []string{
		"application.status.requested", "application.status.preparing", "application.status.needs_input",
		"application.status.ready", "application.status.submitting", "application.status.submitted",
		"application.status.manual_required", "application.status.failed", "application.status.cancelled",
	}, notificationLease, onlyEvent)
	if err != nil || claimed == nil {
		return claimed != nil, err
	}
	eventType = claimed.EventType
	var payload struct {
		ApplicationID string `json:"application_id"`
	}
	status := strings.TrimPrefix(claimed.EventType, "application.status.")
	if claimed.AggregateType != "job_application" || !strings.HasPrefix(claimed.EventType, "application.status.") || json.Unmarshal(claimed.Payload, &payload) != nil || uuid.Validate(payload.ApplicationID) != nil || payload.ApplicationID != claimed.AggregateID {
		cause := errors.New("application status event has invalid payload")
		return true, w.outbox.Fail(ctx, claimed.ID, w.workerID, cause)
	}
	var chatID sql.NullInt64
	var title, company, applyURL, errorCode string
	var submittedAt sql.NullTime
	err = w.pool.QueryRow(ctx, `SELECT ta.chat_id,j.title,c.name,j.apply_url,COALESCE(a.last_error_code,''),a.submitted_at
		FROM job_applications a JOIN jobs j ON j.id=a.job_id JOIN companies c ON c.id=j.company_id
		LEFT JOIN telegram_accounts ta ON ta.user_id=a.user_id WHERE a.id=$1`, payload.ApplicationID).
		Scan(&chatID, &title, &company, &applyURL, &errorCode, &submittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, w.outbox.Complete(ctx, claimed.ID, w.workerID)
	}
	if err != nil {
		return true, w.outbox.Retry(ctx, claimed.ID, w.workerID, err)
	}
	if !chatID.Valid || chatID.Int64 == 0 {
		return true, w.outbox.Complete(ctx, claimed.ID, w.workerID)
	}
	text := ""
	var buttons [][]application.Button
	switch status {
	case "requested":
		text = "Application queued\n\n" + html.EscapeString(title) + " · " + html.EscapeString(company)
	case "needs_input":
		text = "More information is required\n\n" + html.EscapeString(title) + " · " + html.EscapeString(company)
		var questionID, question string
		var optionsJSON []byte
		err := w.pool.QueryRow(ctx, `SELECT id::text,question,COALESCE(options,'[]'::jsonb)
			FROM job_application_questions WHERE application_id=$1 AND required AND status='unanswered' ORDER BY created_at,id LIMIT 1`, payload.ApplicationID).
			Scan(&questionID, &question, &optionsJSON)
		if err == nil {
			text += "\n\n" + html.EscapeString(question)
			var options []jobapplicationdomain.Option
			if json.Unmarshal(optionsJSON, &options) == nil {
				for _, option := range options {
					encoded := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprint(option.Value)))
					callback := "application_answer:" + questionID + ":" + encoded
					if len(callback) <= 64 && fmt.Sprint(option.Value) != "" {
						buttons = append(buttons, []application.Button{{Text: option.Label, CallbackData: callback}})
					}
				}
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return true, w.outbox.Retry(ctx, claimed.ID, w.workerID, err)
		}
	case "submitted":
		text = "Application submitted\n\n" + html.EscapeString(title) + " · " + html.EscapeString(company)
		if submittedAt.Valid {
			text += "\nSubmitted at: " + submittedAt.Time.UTC().Format(time.RFC3339)
		}
	case "manual_required":
		text = "Manual action required\n\n" + html.EscapeString(title) + " · " + html.EscapeString(company) + "\nReason: " + html.EscapeString(errorCode)
	case "failed":
		text = "Application could not be submitted automatically\n\n" + html.EscapeString(title) + " · " + html.EscapeString(company) + "\nReason: " + html.EscapeString(errorCode)
	default:
		return true, w.outbox.Complete(ctx, claimed.ID, w.workerID)
	}
	if applyURL != "" {
		buttons = append(buttons, []application.Button{{Text: "Open application", URL: applyURL}})
	}
	if w.bot == nil {
		return true, w.outbox.Retry(ctx, claimed.ID, w.workerID, errors.New("Telegram sender is unavailable"))
	}
	if err := w.bot.SendMessage(ctx, chatID.Int64, text, buttons); err != nil {
		return true, w.outbox.Retry(ctx, claimed.ID, w.workerID, err)
	}
	return true, w.outbox.Complete(ctx, claimed.ID, w.workerID)
}

func (w *Worker) ProcessEvent(ctx context.Context, eventID string) (bool, error) {
	return w.processMatchEvent(ctx, eventID)
}

func (w *Worker) processMatchEvent(ctx context.Context, onlyEvent string) (found bool, retErr error) {
	started := time.Now()
	defer func() {
		if found {
			observability.DefaultMetrics.Add("hireradar_notification_match_events_total", nil, 1)
			observability.DefaultMetrics.ObserveDuration("hireradar_notification_match_processing_duration", nil, time.Since(started))
		}
	}()
	claimed, err := w.outbox.Claim(ctx, w.workerID, []string{"match.created"}, 5*time.Minute, onlyEvent)
	if err != nil || claimed == nil {
		return claimed != nil, err
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		if retryErr := w.outbox.Retry(ctx, claimed.ID, w.workerID, err); retryErr != nil {
			return true, fmt.Errorf("begin match notification event: %v; schedule retry: %w", err, retryErr)
		}
		return true, fmt.Errorf("begin match notification event: %w", err)
	}
	defer tx.Rollback(ctx)
	var payload struct {
		UserID string `json:"user_id"`
		JobID  string `json:"job_id"`
		Score  int    `json:"score"`
	}
	if err := json.Unmarshal(claimed.Payload, &payload); err != nil || uuid.Validate(payload.UserID) != nil || uuid.Validate(payload.JobID) != nil || payload.Score < 0 || payload.Score > 100 {
		cause := fmt.Errorf("match notification event %s has invalid payload", claimed.ID)
		if failErr := w.outbox.Fail(ctx, claimed.ID, w.workerID, cause); failErr != nil {
			return true, fmt.Errorf("%v; mark event failed: %w", cause, failErr)
		}
		return true, cause
	}
	var confidence int
	confidenceErr := tx.QueryRow(ctx, `SELECT confidence FROM user_job_matches WHERE user_id=$1 AND job_id=$2`, payload.UserID, payload.JobID).Scan(&confidence)
	if errors.Is(confidenceErr, pgx.ErrNoRows) {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			return true, fmt.Errorf("rollback stale match notification: %w", err)
		}
		return true, w.outbox.Complete(ctx, claimed.ID, w.workerID)
	}
	if confidenceErr != nil {
		return w.retryEvent(ctx, tx, claimed.ID, confidenceErr)
	}
	preferences, err := readPreferences(ctx, tx, payload.UserID)
	if err != nil {
		return w.retryEvent(ctx, tx, claimed.ID, err)
	}
	var connected bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM telegram_accounts WHERE user_id=$1)`, payload.UserID).Scan(&connected); err != nil {
		return w.retryEvent(ctx, tx, claimed.ID, err)
	}
	var dismissed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_job_feedback WHERE user_id=$1 AND job_id=$2 AND feedback_type IN ('hidden','not_interested','applied'))`, payload.UserID, payload.JobID).Scan(&dismissed); err != nil {
		return w.retryEvent(ctx, tx, claimed.ID, err)
	}
	queued := false
	if preferences.Enabled && payload.Score >= preferences.MinimumScore && connected && !dismissed {
		now := w.now().UTC()
		scheduled := preferences.ScheduleAt(now)
		if !preferences.Immediate && preferences.DigestEnabled {
			scheduled = nextDigestAt(now, preferences.Timezone)
		}
		immediate := !scheduled.After(now)
		tag, err := tx.Exec(ctx, `INSERT INTO notifications(id,user_id,job_id,scheduled_at,match_score,match_confidence)
			VALUES($1,$2,$3,CASE WHEN $5::bool THEN now() ELSE $4 END,$6,$7)
			ON CONFLICT(user_id,job_id,notification_type) DO UPDATE SET status='pending',scheduled_at=EXCLUDED.scheduled_at,attempts=0,last_error=NULL,match_score=EXCLUDED.match_score,match_confidence=EXCLUDED.match_confidence
			WHERE notifications.status='cancelled'`, uuid.NewString(), payload.UserID, payload.JobID, scheduled, immediate, payload.Score, confidence)
		if err != nil {
			return w.retryEvent(ctx, tx, claimed.ID, err)
		}
		queued = tag.RowsAffected() > 0
	}
	if err := tx.Commit(ctx); err != nil {
		if retryErr := w.outbox.Retry(ctx, claimed.ID, w.workerID, err); retryErr != nil {
			return true, fmt.Errorf("commit match notification event: %v; schedule retry: %w", err, retryErr)
		}
		return true, fmt.Errorf("commit match notification event: %w", err)
	}
	if err := w.outbox.Complete(ctx, claimed.ID, w.workerID); err != nil {
		return true, fmt.Errorf("complete match notification event: %w", err)
	}
	if queued {
		observability.DefaultMetrics.Add("hireradar_notifications_queued_total", nil, 1)
	}
	return true, nil
}

func (w *Worker) retryEvent(ctx context.Context, tx pgx.Tx, eventID string, cause error) (bool, error) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return true, fmt.Errorf("rollback match notification event: %w", err)
	}
	if err := w.outbox.Retry(ctx, eventID, w.workerID, cause); err != nil {
		return true, fmt.Errorf("schedule match notification retry: %w", err)
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

func (w *Worker) deliverNext(ctx context.Context) (found bool, retErr error) {
	started := time.Now()
	defer func() {
		if found {
			observability.DefaultMetrics.Add("hireradar_notification_delivery_attempts_total", nil, 1)
			observability.DefaultMetrics.ObserveDuration("hireradar_notification_delivery_duration", nil, time.Since(started))
		}
	}()
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
		WHERE ((n.status='pending' AND n.scheduled_at<=now()) OR (n.status='delivering' AND n.locked_until<=now())) AND j.status='active'
		ORDER BY n.scheduled_at,n.id FOR UPDATE OF n SKIP LOCKED LIMIT 1`).Scan(&item.ID, &item.UserID, &item.JobID, &item.ChatID, &item.Score, &item.Title, &item.Company, &item.Location, &item.ApplyURL, &item.SalaryMinimum, &item.SalaryMaximum, &item.SalaryCurrency, &item.SalaryPeriod, &item.EmploymentTypes, &item.Skills)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim due Telegram notification: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notifications SET status='delivering',locked_by=$2,locked_until=now()+$3::interval WHERE id=$1`, item.ID, w.workerID, notificationLease.String()); err != nil {
		return false, fmt.Errorf("lease Telegram notification: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit Telegram notification claim: %w", err)
	}
	preferences, err := readPreferences(ctx, w.pool, item.UserID)
	if err != nil {
		return w.retryDelivery(ctx, item.ID, err)
	}
	if !preferences.Enabled || item.Score < preferences.MinimumScore {
		if _, err := w.pool.Exec(ctx, `UPDATE notifications SET status='cancelled',locked_by=NULL,locked_until=NULL WHERE id=$1 AND locked_by=$2`, item.ID, w.workerID); err != nil {
			return w.retryDelivery(ctx, item.ID, err)
		}
		return true, nil
	}
	now := w.now().UTC()
	if scheduled := preferences.ScheduleAt(now); scheduled.After(now) {
		if _, err := w.pool.Exec(ctx, `UPDATE notifications SET status='pending',scheduled_at=$3,locked_by=NULL,locked_until=NULL WHERE id=$1 AND locked_by=$2`, item.ID, w.workerID, scheduled); err != nil {
			return w.retryDelivery(ctx, item.ID, err)
		}
		return true, nil
	}
	if preferences.MaxPerDay != nil {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			return w.retryDelivery(ctx, item.ID, err)
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('notification:' || $1,0))`, item.UserID); err != nil {
			_ = tx.Rollback(ctx)
			return w.retryDelivery(ctx, item.ID, err)
		}
		location, err := time.LoadLocation(preferences.Timezone)
		if err != nil {
			_ = tx.Rollback(ctx)
			return w.retryDelivery(ctx, item.ID, err)
		}
		local := now.In(location)
		dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).UTC()
		var sent int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND status='sent' AND sent_at >= $2`, item.UserID, dayStart).Scan(&sent); err != nil {
			_ = tx.Rollback(ctx)
			return w.retryDelivery(ctx, item.ID, err)
		}
		if sent >= *preferences.MaxPerDay {
			nextDay := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location).UTC()
			if _, err := tx.Exec(ctx, `UPDATE notifications SET status='pending',scheduled_at=$3,locked_by=NULL,locked_until=NULL WHERE id=$1 AND locked_by=$2`, item.ID, w.workerID, nextDay); err != nil {
				_ = tx.Rollback(ctx)
				return w.retryDelivery(ctx, item.ID, err)
			}
			if err := tx.Commit(ctx); err != nil {
				return w.retryDelivery(ctx, item.ID, err)
			}
			return true, nil
		}
		if err := tx.Commit(ctx); err != nil {
			return w.retryDelivery(ctx, item.ID, err)
		}
	}
	if _, err := w.pool.Exec(ctx, `UPDATE notifications SET attempts=attempts+1 WHERE id=$1 AND status='delivering' AND locked_by=$2`, item.ID, w.workerID); err != nil {
		return w.retryDelivery(ctx, item.ID, err)
	}
	if w.bot == nil {
		return w.retryDelivery(ctx, item.ID, errors.New("Telegram sender is unavailable"))
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
		{{Text: "Apply", CallbackData: "job:apply:" + item.JobID}, {Text: "Open", URL: w.openNotificationURL(item)}},
		{{Text: "Save", CallbackData: "job:save:" + item.JobID}, {Text: "Hide", CallbackData: "job:hide:" + item.JobID}},
	}
	if err := w.bot.SendMessage(ctx, item.ChatID, text, buttons); err != nil {
		return w.retryDelivery(ctx, item.ID, err)
	}
	if _, err := w.pool.Exec(ctx, `UPDATE notifications SET status='sent',sent_at=now(),last_error=NULL,locked_by=NULL,locked_until=NULL WHERE id=$1 AND status='delivering' AND locked_by=$2`, item.ID, w.workerID); err != nil {
		return true, fmt.Errorf("mark Telegram notification sent: %w", err)
	}
	observability.DefaultMetrics.Add("hireradar_notifications_sent_total", nil, 1)
	return true, nil
}

func (w *Worker) openNotificationURL(item dueNotification) string {
	if w.openURLBase == "" {
		return item.ApplyURL
	}
	return w.openURLBase + "/api/v1/notifications/open/" + item.ID
}

func (w *Worker) retryDelivery(ctx context.Context, id string, cause error) (bool, error) {
	const safeMessage = "Telegram delivery failed"
	var status string
	err := w.pool.QueryRow(ctx, `UPDATE notifications
		SET status=CASE WHEN attempts >= $3 THEN 'failed' ELSE 'pending' END,
		    last_error=$4,
		    scheduled_at=CASE WHEN attempts >= $3 THEN scheduled_at ELSE now()+LEAST(3600,POWER(2,LEAST(attempts,12))::int)*interval '1 second' END,
		    locked_by=NULL,locked_until=NULL
		WHERE id=$1 AND status='delivering' AND locked_by=$2 RETURNING status`, id, w.workerID, maxDeliveryAttempts, safeMessage).Scan(&status)
	if err != nil {
		return true, fmt.Errorf("schedule Telegram retry: %w", err)
	}
	if status == "failed" {
		observability.DefaultMetrics.Add("hireradar_notifications_failed_total", nil, 1)
		return true, errors.New("Telegram delivery failed; retry limit reached")
	}
	observability.DefaultMetrics.Add("hireradar_notifications_retry_total", nil, 1)
	return true, errors.New("Telegram delivery failed; retry scheduled")
}

func readPreferences(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID string) (domain.NotificationPreferences, error) {
	prefs := domain.DefaultPreferences()
	var start, end sql.NullString
	var maxPerDay sql.NullInt64
	err := db.QueryRow(ctx, `SELECT enabled,minimum_score,immediate,digest_enabled,timezone,quiet_start::text,quiet_end::text,max_per_day FROM notification_preferences WHERE user_id=$1`, userID).Scan(&prefs.Enabled, &prefs.MinimumScore, &prefs.Immediate, &prefs.DigestEnabled, &prefs.Timezone, &start, &end, &maxPerDay)
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
