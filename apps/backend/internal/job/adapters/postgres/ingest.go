package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/Hell077/HireRadar/apps/backend/internal/job/normalization"
	sourcedomain "github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Ingestor struct{}

func NewIngestor() *Ingestor { return &Ingestor{} }

func (i *Ingestor) Save(ctx context.Context, tx pgx.Tx, source sourcedomain.Source, runID string, external sourcedomain.ExternalJob) (created, updated bool, err error) {
	job := normalization.Normalize(source, external)
	companyKey := normalization.CompanyKey(job.Company)
	if companyKey == "" {
		companyKey = "unknown " + source.ID
		job.Company = "Unknown company"
	}
	if job.NormalizedTitle == "" || job.ApplyURL == "" {
		return false, false, errors.New("normalized job is missing title or apply URL")
	}
	canonical := CanonicalURL(job.ApplyURL)
	locationKey := strings.ToLower(strings.Join(strings.Fields(job.Location), " "))
	fingerprint := sha256.Sum256([]byte(companyKey + "\x00" + job.NormalizedTitle + "\x00" + locationKey))
	companyID, err := i.company(ctx, tx, job.Company, companyKey)
	if err != nil {
		return false, false, err
	}
	jobID, priority, sameSource, found, err := i.resolve(ctx, tx, source.ID, external.ExternalID, canonical, fingerprint[:])
	if err != nil {
		return false, false, err
	}
	if !found {
		jobID = uuid.NewString()
		_, err = tx.Exec(ctx, `INSERT INTO jobs(id,company_id,title,normalized_title,description,employment_types,remote_policy,location,location_countries,eligibility,apply_url,canonical_url,published_at,fingerprint,status,source_priority) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,''),$13,$14,'active',$15)`, jobID, companyID, job.Title, job.NormalizedTitle, job.Description, job.EmploymentTypes, string(job.RemotePolicy), job.Location, job.Countries, string(job.Eligibility), job.ApplyURL, canonical, job.PublishedAt, fingerprint[:], source.Priority)
		if err != nil {
			return false, false, fmt.Errorf("insert normalized job: %w", err)
		}
		created = true
	} else {
		allowReplace := sameSource || source.Priority <= priority
		if allowReplace {
			var before struct {
				title, description, location, applyURL string
				remotePolicy, eligibility, status      string
				employmentTypes, countries             []string
				publishedAt                            *time.Time
			}
			if err := tx.QueryRow(ctx, `SELECT title,description,location,apply_url,remote_policy,eligibility,status,employment_types,location_countries,published_at FROM jobs WHERE id=$1 FOR UPDATE`, jobID).Scan(&before.title, &before.description, &before.location, &before.applyURL, &before.remotePolicy, &before.eligibility, &before.status, &before.employmentTypes, &before.countries, &before.publishedAt); err != nil {
				return false, false, err
			}
			updated = before.title != job.Title || before.description != job.Description || before.location != job.Location || before.applyURL != job.ApplyURL || before.remotePolicy != string(job.RemotePolicy) || before.eligibility != string(job.Eligibility) || before.status != string(jobdomain.Active) || strings.Join(before.employmentTypes, "\x00") != strings.Join(job.EmploymentTypes, "\x00") || strings.Join(before.countries, "\x00") != strings.Join(job.Countries, "\x00") || !sameTime(before.publishedAt, job.PublishedAt)
			_, err = tx.Exec(ctx, `UPDATE jobs SET company_id=$2,title=$3,normalized_title=$4,description=$5,employment_types=$6,remote_policy=$7,location=$8,location_countries=$9,eligibility=$10,apply_url=$11,canonical_url=NULLIF($12,''),published_at=$13,fingerprint=$14,source_priority=$15,status='active',closed_at=NULL,last_seen_at=now(),updated_at=CASE WHEN $16 THEN now() ELSE updated_at END WHERE id=$1`, jobID, companyID, job.Title, job.NormalizedTitle, job.Description, job.EmploymentTypes, string(job.RemotePolicy), job.Location, job.Countries, string(job.Eligibility), job.ApplyURL, canonical, job.PublishedAt, fingerprint[:], minPriority(priority, source.Priority), updated)
			if err != nil {
				return false, false, fmt.Errorf("update normalized job: %w", err)
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE jobs SET status='active',closed_at=NULL,last_seen_at=now() WHERE id=$1`, jobID)
			if err != nil {
				return false, false, err
			}
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_sources(source_id,external_id,job_id,url,last_seen_run_id,missing_count,is_active) VALUES($1,$2,$3,$4,$5,0,true) ON CONFLICT(source_id,external_id) DO UPDATE SET job_id=EXCLUDED.job_id,url=EXCLUDED.url,last_seen_at=now(),last_seen_run_id=EXCLUDED.last_seen_run_id,missing_count=0,is_active=true`, source.ID, external.ExternalID, jobID, job.ApplyURL, runID)
	if err != nil {
		return false, false, fmt.Errorf("save job source reference: %w", err)
	}
	if created || updated {
		eventType := "job.updated"
		if created {
			eventType = "job.created"
		}
		payload, _ := json.Marshal(map[string]string{"job_id": jobID, "company_id": companyID, "source_id": source.ID})
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,$2,'job',$3,$4::jsonb)`, uuid.NewString(), eventType, jobID, payload); err != nil {
			return false, false, fmt.Errorf("publish job event: %w", err)
		}
	}
	return created, updated, nil
}

func (i *Ingestor) CloseMissing(ctx context.Context, tx pgx.Tx, sourceID, runID string) error {
	if _, err := tx.Exec(ctx, `UPDATE job_sources SET missing_count=missing_count+1,is_active=(missing_count+1<2) WHERE source_id=$1 AND is_active AND last_seen_run_id IS DISTINCT FROM $2`, sourceID, runID); err != nil {
		return fmt.Errorf("increment missing job references: %w", err)
	}
	rows, err := tx.Query(ctx, `UPDATE jobs j SET status='closed',closed_at=now(),updated_at=now() WHERE j.status='active' AND EXISTS(SELECT 1 FROM job_sources js WHERE js.job_id=j.id AND js.source_id=$1 AND js.missing_count>=2 AND NOT js.is_active) AND NOT EXISTS(SELECT 1 FROM job_sources active WHERE active.job_id=j.id AND active.is_active) RETURNING j.id`, sourceID)
	if err != nil {
		return fmt.Errorf("close jobs missing from all sources: %w", err)
	}
	closedIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		closedIDs = append(closedIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range closedIDs {
		payload, _ := json.Marshal(map[string]string{"job_id": id, "source_id": sourceID})
		if _, err := tx.Exec(ctx, `INSERT INTO outbox_events(id,event_type,aggregate_type,aggregate_id,payload) VALUES($1,'job.closed','job',$2,$3::jsonb)`, uuid.NewString(), id, payload); err != nil {
			return fmt.Errorf("publish closed job event: %w", err)
		}
	}
	return nil
}

func (i *Ingestor) company(ctx context.Context, tx pgx.Tx, name, key string) (string, error) {
	id := uuid.NewString()
	err := tx.QueryRow(ctx, `INSERT INTO companies(id,name,normalized_name) VALUES($1,$2,$3) ON CONFLICT(normalized_name) DO UPDATE SET updated_at=companies.updated_at RETURNING id`, id, name, key).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("resolve company: %w", err)
	}
	return id, nil
}

func (i *Ingestor) resolve(ctx context.Context, tx pgx.Tx, sourceID, externalID, canonical string, fingerprint []byte) (string, int, bool, bool, error) {
	var id string
	var priority int
	err := tx.QueryRow(ctx, `SELECT j.id,j.source_priority FROM job_sources js JOIN jobs j ON j.id=js.job_id WHERE js.source_id=$1 AND js.external_id=$2`, sourceID, externalID).Scan(&id, &priority)
	if err == nil {
		return id, priority, true, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", 0, false, false, err
	}
	queries := []struct {
		query string
		arg   any
	}{{`SELECT id,source_priority FROM jobs WHERE canonical_url=$1 ORDER BY source_priority,id LIMIT 1`, canonical}, {`SELECT id,source_priority FROM jobs WHERE fingerprint=$1 ORDER BY source_priority,id LIMIT 1`, fingerprint}}
	for _, candidate := range queries {
		if candidate.query == queries[0].query && canonical == "" {
			continue
		}
		err = tx.QueryRow(ctx, candidate.query, candidate.arg).Scan(&id, &priority)
		if err == nil {
			return id, priority, false, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", 0, false, false, err
		}
	}
	return "", 0, false, false, nil
}

func CanonicalURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		if strings.HasPrefix(strings.ToLower(key), "utm_") || strings.EqualFold(key, "ref") || strings.EqualFold(key, "source") {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String()
}
func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
func minPriority(a, b int) int {
	if b < a {
		return b
	}
	return a
}
