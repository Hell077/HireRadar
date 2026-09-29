package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	greenhouse "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/adapters/greenhouse"
	jobapplicationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/adapters/postgres"
	jobapplicationapp "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/application"
	jobapplication "github.com/Hell077/HireRadar/apps/backend/internal/jobapplication/domain"
	notificationpostgres "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/postgres"
	telegram "github.com/Hell077/HireRadar/apps/backend/internal/notification/adapters/telegram"
	notificationapp "github.com/Hell077/HireRadar/apps/backend/internal/notification/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/notification/telegramwebhook"
	outboxpostgres "github.com/Hell077/HireRadar/apps/backend/internal/outbox/adapters/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type resumeReader struct{}

func (resumeReader) ReadResume(context.Context, string, string) (greenhouse.ResumeAttachment, error) {
	return greenhouse.ResumeAttachment{}, nil
}

func TestTelegramApplyRunsThroughGreenhouseAndConfirmsSubmittedOnce(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated PostgreSQL test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	userID, companyID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	telegramUserID := time.Now().UnixNano()
	companyName := "Application E2E " + companyID
	title := "Backend Engineer " + jobID
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'test')`, userID, userID+"@e2e.example"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	if _, err := pool.Exec(ctx, `INSERT INTO companies(id,name,normalized_name) VALUES($1,$2,$3)`, companyID, companyName, companyID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM companies WHERE id=$1`, companyID)
	if _, err := pool.Exec(ctx, `INSERT INTO jobs(id,company_id,title,normalized_title,remote_policy,eligibility,apply_url,fingerprint,status) VALUES($1,$2,$3,'backend engineer','unknown','unknown','https://boards.greenhouse.io/acme/jobs/123',$4,'active')`, jobID, companyID, title, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM jobs WHERE id=$1`, jobID)
	if _, err := pool.Exec(ctx, `INSERT INTO user_job_matches(user_id,job_id,score,confidence,candidate_version,job_version) VALUES($1,$2,90,85,0,1)`, userID, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO telegram_accounts(id,user_id,telegram_user_id,chat_id) VALUES($1,$2,$3,$3)`, uuid.NewString(), userID, telegramUserID); err != nil {
		t.Fatal(err)
	}
	profiles := jobapplicationpostgres.NewProfileStore(pool)
	if err := profiles.SaveProfile(ctx, jobapplication.ApplicationProfile{UserID: userID, FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.test", WorkAuthorization: []jobapplication.WorkAuthorization{}, CustomAnswers: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM application_profiles WHERE user_id=$1`, userID)

	store := jobapplicationpostgres.NewStore(pool)
	var telegramMethods []string
	var telegramMessages []string
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/bottest-token/")
		telegramMethods = append(telegramMethods, method)
		var payload struct {
			Text string `json:"text"`
		}
		if method == "sendMessage" {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode Telegram sendMessage: %v", err)
			}
			telegramMessages = append(telegramMessages, payload.Text)
		} else {
			_, _ = io.Copy(io.Discard, r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	defer telegramServer.Close()
	telegramBot := telegram.NewClientWithBaseURL("test-token", telegramServer.URL, telegramServer.Client())

	var prepareCalls, submissionCalls int
	atsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/boards/acme/jobs/123" {
			t.Errorf("unexpected Greenhouse request path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			prepareCalls++
			if r.URL.Query().Get("questions") != "true" {
				t.Errorf("Greenhouse prepare did not request application questions")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"questions":[{"required":true,"label":"First Name","fields":[{"name":"first_name","type":"input_text"}]},{"required":true,"label":"Email","fields":[{"name":"email","type":"input_text"}]}]}`)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected Greenhouse method %q", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		submissionCalls++
		apiKey, password, ok := r.BasicAuth()
		if !ok || apiKey != "test-employer-key" || password != "" {
			t.Errorf("Greenhouse credentials were not sent with the submission")
		}
		var answers map[string]any
		if err := json.NewDecoder(r.Body).Decode(&answers); err != nil {
			t.Errorf("decode Greenhouse submission: %v", err)
		}
		if answers["first_name"] != "Ada" || answers["email"] != "ada@example.test" {
			t.Errorf("prepared candidate values missing from submission: %+v", answers)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer atsServer.Close()
	provider := greenhouse.NewWithHTTPClient(greenhouse.CredentialMap{"acme": "test-employer-key"}, resumeReader{}, atsServer.Client(), atsServer.URL)

	applicationService := jobapplicationapp.NewRequestService(store, uuid.NewString, time.Now)
	notificationService := notificationapp.NewService(notificationpostgres.NewStore(pool), "hireradar_bot", time.Now, telegramBot)
	notificationService.SetApplicationRequester(applicationService)
	webhook := httptest.NewServer(telegramwebhook.New("integration-webhook-secret", notificationService, pool))
	defer webhook.Close()
	callbackBody := `{"callback_query":{"id":"apply-e2e","data":"job:apply:` + jobID + `","from":{"id":` + fmt.Sprint(telegramUserID) + `}}}`
	for range 2 {
		request, err := http.NewRequest(http.MethodPost, webhook.URL+"/telegram/webhook", strings.NewReader(callbackBody))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "integration-webhook-secret")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("Telegram Apply webhook status=%d", response.StatusCode)
		}
	}

	var applicationID, requestEventID string
	var applications, requestedEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*),min(id::text) FROM job_applications WHERE user_id=$1 AND job_id=$2`, userID, jobID).Scan(&applications, &applicationID); err != nil || applications != 1 {
		t.Fatalf("applications=%d id=%q err=%v", applications, applicationID, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*),min(id::text) FROM outbox_events WHERE event_type='application.requested' AND aggregate_id=$1`, applicationID).Scan(&requestedEvents, &requestEventID); err != nil || requestedEvents != 1 {
		t.Fatalf("application.requested events=%d err=%v", requestedEvents, err)
	}

	worker := jobapplicationapp.NewWorker(outboxpostgres.NewStore(pool), store, jobapplicationapp.NewProviderRegistry(provider), "application-e2e", time.Now)
	if processed, err := worker.ProcessEvent(ctx, requestEventID); err != nil || !processed {
		t.Fatalf("application worker processed=%v err=%v", processed, err)
	}
	var status string
	var submittedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT status,submitted_at FROM job_applications WHERE id=$1`, applicationID).Scan(&status, &submittedAt); err != nil {
		t.Fatal(err)
	}
	if status != string(jobapplication.StatusSubmitted) || submittedAt == nil || submissionCalls != 1 || prepareCalls != 1 {
		t.Fatalf("status=%q submitted_at=%v prepares=%d submissions=%d", status, submittedAt, prepareCalls, submissionCalls)
	}
	var attempts, processedEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM job_application_attempts WHERE application_id=$1`, applicationID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='application.requested' AND aggregate_id=$1 AND status='processed'`, applicationID).Scan(&processedEvents); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || processedEvents != 1 {
		t.Fatalf("attempts=%d processed request events=%d", attempts, processedEvents)
	}
	if processed, err := worker.ProcessEvent(ctx, requestEventID); err != nil || processed {
		t.Fatalf("duplicate application work found=%v err=%v", processed, err)
	}

	notificationWorker := notificationpostgres.NewWorker(pool, telegramBot, time.Now)
	statusRows, err := pool.Query(ctx, `SELECT id::text FROM outbox_events WHERE event_type LIKE 'application.status.%' AND aggregate_id=$1 AND status='pending' ORDER BY occurred_at,id`, applicationID)
	if err != nil {
		t.Fatal(err)
	}
	var statusEventIDs []string
	for statusRows.Next() {
		var id string
		if err := statusRows.Scan(&id); err != nil {
			statusRows.Close()
			t.Fatal(err)
		}
		statusEventIDs = append(statusEventIDs, id)
	}
	if err := statusRows.Err(); err != nil {
		statusRows.Close()
		t.Fatal(err)
	}
	statusRows.Close()
	for _, eventID := range statusEventIDs {
		processed, err := notificationWorker.ProcessApplicationEvent(ctx, eventID)
		if err != nil || !processed {
			t.Fatalf("application status event %s processed=%v err=%v", eventID, processed, err)
		}
	}
	var submittedStatusEvents, pendingApplicationEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type='application.status.submitted' AND aggregate_id=$1 AND status='processed'`, applicationID).Scan(&submittedStatusEvents); err != nil || submittedStatusEvents != 1 {
		t.Fatalf("processed submitted status events=%d err=%v", submittedStatusEvents, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND (event_type='application.requested' OR event_type LIKE 'application.status.%') AND status IN ('pending','processing')`, applicationID).Scan(&pendingApplicationEvents); err != nil || pendingApplicationEvents != 0 {
		t.Fatalf("pending application events=%d err=%v", pendingApplicationEvents, err)
	}
	confirmed := false
	for _, message := range telegramMessages {
		confirmed = confirmed || strings.Contains(message, "Application submitted")
	}
	if !confirmed {
		t.Fatalf("Telegram messages did not include submitted confirmation: %+v", telegramMessages)
	}
	if len(telegramMethods) < 3 || telegramMethods[0] != "answerCallbackQuery" || telegramMethods[1] != "answerCallbackQuery" {
		t.Fatalf("Telegram callback acknowledgement calls=%v", telegramMethods)
	}
}
