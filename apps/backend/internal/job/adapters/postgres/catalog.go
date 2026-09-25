package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	jobdomain "github.com/Hell077/HireRadar/apps/backend/internal/job/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Catalog struct{ pool *pgxpool.Pool }
type ListQuery struct {
	Cursor, Status, RemotePolicy, Country, SourceID, Eligibility string
	Limit                                                        int
}
type ListResult struct {
	Items      []jobdomain.Job `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

var sourceSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,99}$`)

func NewCatalog(pool *pgxpool.Pool) *Catalog { return &Catalog{pool: pool} }

func (c *Catalog) List(ctx context.Context, q ListQuery) (ListResult, error) {
	if q.Limit == 0 {
		q.Limit = 25
	}
	if q.Limit < 1 || q.Limit > 100 {
		return ListResult{}, fmt.Errorf("%w: limit must be between 1 and 100", jobdomain.ErrInvalidQuery)
	}
	if q.Status == "" {
		q.Status = string(jobdomain.Active)
	}
	if q.Status != "active" && q.Status != "closed" && q.Status != "expired" && q.Status != "unknown" {
		return ListResult{}, fmt.Errorf("%w: invalid job status", jobdomain.ErrInvalidQuery)
	}
	if q.RemotePolicy != "" && !oneOf(q.RemotePolicy, "worldwide", "remote", "remote_region", "remote_country", "hybrid", "onsite", "unknown") {
		return ListResult{}, fmt.Errorf("%w: invalid remote policy", jobdomain.ErrInvalidQuery)
	}
	if q.Eligibility != "" && !oneOf(q.Eligibility, "eligible", "not_eligible", "unknown") {
		return ListResult{}, fmt.Errorf("%w: invalid eligibility", jobdomain.ErrInvalidQuery)
	}
	if q.Country != "" {
		q.Country = strings.ToUpper(strings.TrimSpace(q.Country))
		if len(q.Country) != 2 || !asciiLetters(q.Country) {
			return ListResult{}, fmt.Errorf("%w: country must be a two-letter code", jobdomain.ErrInvalidQuery)
		}
	}
	if q.SourceID != "" && !sourceSlug.MatchString(q.SourceID) {
		return ListResult{}, fmt.Errorf("%w: invalid source ID", jobdomain.ErrInvalidQuery)
	}
	args := []any{q.Status}
	conditions := []string{"j.status=$1"}
	add := func(value any, condition string) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if q.RemotePolicy != "" {
		add(q.RemotePolicy, "j.remote_policy=$%d")
	}
	if q.Country != "" {
		add(q.Country, "$%d=ANY(j.location_countries)")
	}
	if q.Eligibility != "" {
		add(q.Eligibility, "j.eligibility=$%d")
	}
	if q.SourceID != "" {
		add(q.SourceID, "EXISTS(SELECT 1 FROM job_sources js WHERE js.job_id=j.id AND js.source_id=$%d AND js.is_active)")
	}
	if q.Cursor != "" {
		created, id, err := decodeCursor(q.Cursor)
		if err != nil {
			return ListResult{}, jobdomain.ErrInvalidCursor
		}
		args = append(args, created, id)
		n := len(args) - 1
		conditions = append(conditions, fmt.Sprintf("(j.created_at,j.id)<($%d,$%d::uuid)", n, n+1))
	}
	args = append(args, q.Limit+1)
	query := `SELECT j.id::text,j.company_id::text,c.name,j.title,j.normalized_title,j.description,j.employment_types,j.remote_policy,j.location,j.location_countries,j.eligibility,j.apply_url,j.published_at,j.first_seen_at,j.last_seen_at,j.status,j.source_priority,j.created_at,j.updated_at
		FROM jobs j JOIN companies c ON c.id=j.company_id WHERE ` + strings.Join(conditions, " AND ") + fmt.Sprintf(" ORDER BY j.created_at DESC,j.id DESC LIMIT $%d", len(args))
	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	items := []jobdomain.Job{}
	for rows.Next() {
		var item jobdomain.Job
		if err := rows.Scan(&item.ID, &item.CompanyID, &item.Company, &item.Title, &item.NormalizedTitle, &item.Description, &item.EmploymentTypes, &item.RemotePolicy, &item.Location, &item.Countries, &item.Eligibility, &item.ApplyURL, &item.PublishedAt, &item.FirstSeenAt, &item.LastSeenAt, &item.Status, &item.SourcePriority, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	result := ListResult{Items: items}
	if len(items) > q.Limit {
		result.Items = items[:q.Limit]
		last := result.Items[len(result.Items)-1]
		result.NextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return result, nil
}

func (c *Catalog) Get(ctx context.Context, id string) (jobdomain.Job, error) {
	if _, err := uuid.Parse(id); err != nil {
		return jobdomain.Job{}, jobdomain.ErrInvalidQuery
	}
	var item jobdomain.Job
	err := c.pool.QueryRow(ctx, `SELECT j.id::text,j.company_id::text,c.name,j.title,j.normalized_title,j.description,j.employment_types,j.remote_policy,j.location,j.location_countries,j.eligibility,j.apply_url,j.published_at,j.first_seen_at,j.last_seen_at,j.status,j.source_priority,j.created_at,j.updated_at FROM jobs j JOIN companies c ON c.id=j.company_id WHERE j.id=$1`, id).Scan(&item.ID, &item.CompanyID, &item.Company, &item.Title, &item.NormalizedTitle, &item.Description, &item.EmploymentTypes, &item.RemotePolicy, &item.Location, &item.Countries, &item.Eligibility, &item.ApplyURL, &item.PublishedAt, &item.FirstSeenAt, &item.LastSeenAt, &item.Status, &item.SourcePriority, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return jobdomain.Job{}, pgx.ErrNoRows
	}
	if err != nil {
		return jobdomain.Job{}, fmt.Errorf("get job: %w", err)
	}
	return item, nil
}

func encodeCursor(created time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(created.UTC().Format(time.RFC3339Nano) + "|" + id))
}
func decodeCursor(value string) (time.Time, string, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.Split(string(data), "|")
	if len(parts) != 2 {
		return time.Time{}, "", jobdomain.ErrInvalidCursor
	}
	created, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return time.Time{}, "", err
	}
	return created, parts[1], nil
}
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
func asciiLetters(value string) bool {
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
