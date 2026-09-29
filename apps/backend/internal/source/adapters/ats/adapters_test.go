package ats

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/source/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGreenhouseStableIDAndRawPayload(t *testing.T) {
	const body = `{"jobs":[{"id":42,"title":"Backend Engineer","content":"<p>Go</p>","absolute_url":"https://jobs.test/42","updated_at":"2026-01-02T03:04:05Z","location":{"name":"Remote"}}]}`
	r := NewRegistry()
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/boards/acme/jobs" || req.URL.Query().Get("content") != "true" {
			t.Fatalf("unexpected Greenhouse request: %s", req.URL)
		}
		return response(body), nil
	})
	got, err := r.Fetch(context.Background(), domain.Source{Type: domain.Greenhouse, CompanyName: "Acme", Config: []byte(`{"board":"acme"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].ExternalID != "42" || got.Jobs[0].Title != "Backend Engineer" || !strings.Contains(string(got.Jobs[0].Raw), "\"id\":42") {
		t.Fatalf("unexpected Greenhouse job: %#v", got.Jobs)
	}
}

func TestLeverPaginatesUntilShortPage(t *testing.T) {
	requests := 0
	r := NewRegistry()
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Query().Get("limit") != "1" {
			t.Fatalf("expected page size 1: %s", req.URL)
		}
		if req.URL.Query().Get("skip") == "0" {
			return response(`[ {"id":"post-1","text":"Engineer","hostedUrl":"https://jobs.test/1","createdAt":1760000000000,"categories":{"location":"Remote"}} ]`), nil
		}
		return response(`[]`), nil
	})
	got, err := r.Fetch(context.Background(), domain.Source{Type: domain.Lever, CompanyName: "Acme", Config: []byte(`{"board":"acme","page_size":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(got.Jobs) != 1 || got.Jobs[0].ExternalID != "post-1" || !strings.Contains(string(got.Jobs[0].Raw), "createdAt") {
		t.Fatalf("unexpected Lever result after %d requests: %#v", requests, got)
	}
}

func TestAshbyMapsPublicBoardPostings(t *testing.T) {
	const body = `{"apiVersion":"1","jobs":[{"id":"ash-1","title":"SRE","descriptionPlain":"Build systems","jobUrl":"https://jobs.test/1","location":"Remote","employmentType":"FullTime","publishedAt":"2026-02-03T10:00:00Z"}]}`
	r := NewRegistry()
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/posting-api/job-board/acme" || req.URL.Query().Get("includeCompensation") != "true" {
			t.Fatalf("unexpected Ashby request: %s", req.URL)
		}
		return response(body), nil
	})
	got, err := r.Fetch(context.Background(), domain.Source{Type: domain.Ashby, CompanyName: "Acme", Config: []byte(`{"board":"acme"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].ExternalID != "ash-1" || got.Jobs[0].EmploymentType != "FullTime" || got.Jobs[0].PublishedAt == nil || !strings.Contains(string(got.Jobs[0].Raw), "descriptionPlain") {
		t.Fatalf("unexpected Ashby job: %#v", got)
	}
}

func TestWorkablePublicCareersFeed(t *testing.T) {
	const body = `{"jobs":[{"title":"Senior Engineer","shortcode":"AB12CD34","application_url":"https://apply.workable.com/j/AB12CD34/apply","employment_type":"Full-time","telecommuting":true,"country":"France","city":"Paris","published_on":"2026-07-30","description":"<p>Build systems</p>","locations":[{"countryCode":"FR"}]}]}`
	r := NewRegistry()
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "www.workable.com" || req.URL.Path != "/api/accounts/huggingface" || req.URL.Query().Get("details") != "true" {
			t.Fatalf("unexpected Workable request: %s", req.URL)
		}
		return response(body), nil
	})
	got, err := r.Fetch(context.Background(), domain.Source{Type: domain.Workable, CompanyName: "Hugging Face", Config: []byte(`{"board":"huggingface"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != domain.SyncSnapshot || len(got.Jobs) != 1 {
		t.Fatalf("unexpected Workable fetch result: %+v", got)
	}
	job := got.Jobs[0]
	if job.ExternalID != "AB12CD34" || job.EmploymentType != "Full-time" || !job.Remote || len(job.Countries) != 1 || job.Countries[0] != "FR" || job.PublishedAt == nil || !strings.Contains(string(job.Raw), "telecommuting") {
		t.Fatalf("unexpected Workable job mapping: %+v", job)
	}
}

func TestGitHubPaginatesAndExcludesPullRequests(t *testing.T) {
	requests := 0
	r := NewRegistryWithGitHubToken("test-github-token")
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/repos/hireradar/jobs/issues" || req.URL.Query().Get("state") != "open" || req.Header.Get("User-Agent") == "" {
			t.Fatalf("unexpected GitHub request: %s", req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer test-github-token" {
			t.Fatalf("authorization header = %q", req.Header.Get("Authorization"))
		}
		if req.URL.Query().Get("labels") != "hiring,backend" {
			t.Fatalf("labels = %q", req.URL.Query().Get("labels"))
		}
		if req.URL.Query().Get("page") == "1" {
			return response(`[{"number":9,"title":"Go Developer","body":"Apply","html_url":"https://github.com/hireradar/jobs/issues/9","created_at":"2026-01-02T03:04:05Z"},{"number":10,"title":"PR","html_url":"https://github.com/hireradar/jobs/pull/10","pull_request":{"url":"https://api.github.com/repos/hireradar/jobs/pulls/10"}}]`), nil
		}
		return response(`[]`), nil
	})
	got, err := r.Fetch(context.Background(), domain.Source{Type: domain.GitHub, CompanyName: "HireRadar", Config: []byte(`{"owner":"hireradar","repo":"jobs","labels":["hiring","backend"],"page_size":2}`)})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(got.Jobs) != 1 || got.Jobs[0].ExternalID != "9" || got.Jobs[0].PublishedAt == nil || len(got.NextCursor) == 0 {
		t.Fatalf("unexpected GitHub result after %d requests: %#v", requests, got)
	}
	if !strings.Contains(string(got.Jobs[0].Raw), "Go Developer") {
		t.Fatalf("raw issue not preserved: %s", got.Jobs[0].Raw)
	}
}

func TestGitHubIssueJobClassifierKeepsPostingsAndSkipsBugs(t *testing.T) {
	if !IsJobPosting("Hiring: Senior Go Engineer", "Remote worldwide. Apply with your CV.") {
		t.Fatal("expected hiring issue to be a job posting")
	}
	if IsJobPosting("Fix engineer settings page", "The developer console crashes when opening settings.") {
		t.Fatal("software bug issue was classified as a vacancy")
	}
}

func TestRejectsBadBoardAndNonSuccessfulResponse(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Fetch(context.Background(), domain.Source{Type: domain.Greenhouse, Config: []byte(`{"board":"../private"}`)}); err == nil {
		t.Fatal("expected invalid board to fail")
	}
	r.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("slow down")), Header: make(http.Header)}, nil
	})
	if _, err := r.Fetch(context.Background(), domain.Source{Type: domain.Greenhouse, Config: []byte(`{"board":"acme"}`)}); err == nil || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("expected HTTP error, got %v", err)
	}
}

func TestRetryAfterParserSupportsSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	if got := retryAfter("120", now); got != 2*time.Minute {
		t.Fatalf("seconds Retry-After=%s", got)
	}
	if got := retryAfter(now.Add(90*time.Second).Format(http.TimeFormat), now); got != 90*time.Second {
		t.Fatalf("date Retry-After=%s", got)
	}
}

func TestATSBoardsDistinguishValidEmptySnapshotsFromInvalidPayloads(t *testing.T) {
	tests := []struct {
		name    string
		typ     domain.Type
		config  string
		path    string
		body    string
		wantErr bool
	}{
		{name: "empty greenhouse board", typ: domain.Greenhouse, config: `{"board":"acme"}`, path: "/v1/boards/acme/jobs", body: `{"jobs":[]}`},
		{name: "invalid greenhouse payload", typ: domain.Greenhouse, config: `{"board":"acme"}`, path: "/v1/boards/acme/jobs", body: `{"message":"not a board"}`, wantErr: true},
		{name: "empty ashby board", typ: domain.Ashby, config: `{"board":"acme"}`, path: "/posting-api/job-board/acme", body: `{"jobs":[]}`},
		{name: "invalid ashby payload", typ: domain.Ashby, config: `{"board":"acme"}`, path: "/posting-api/job-board/acme", body: `{"jobs":null}`, wantErr: true},
		{name: "invalid lever payload", typ: domain.Lever, config: `{"board":"acme","page_size":1}`, path: "/v0/postings/acme", body: `null`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := NewRegistry()
			r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != test.path {
					t.Fatalf("unexpected path %s", req.URL.Path)
				}
				return response(test.body), nil
			})
			got, err := r.Fetch(context.Background(), domain.Source{Type: test.typ, CompanyName: "Acme", Config: []byte(test.config)})
			if test.wantErr && err == nil {
				t.Fatal("expected invalid payload error")
			}
			if !test.wantErr && (err != nil || len(got.Jobs) != 0) {
				t.Fatalf("valid empty snapshot returned jobs=%d err=%v", len(got.Jobs), err)
			}
		})
	}
}

func TestPublicRollingFeedsParseRealisticPayloadsWithoutClaimingSnapshot(t *testing.T) {
	tests := []struct {
		name  string
		typ   domain.Type
		body  string
		check func(domain.ExternalJob) bool
	}{
		{name: "RemoteOK", typ: domain.RemoteOK, body: `[{"legal":"Public API"},{"id":123,"position":"Senior Go Engineer","company":"Acme","url":"https://remoteok.com/remote-jobs/123","location":"Worldwide","date":"2026-09-28T12:00:00Z","description":"Build Go services"}]`, check: func(j domain.ExternalJob) bool {
			return j.ExternalID == "123" && j.Title == "Senior Go Engineer" && j.CompanyName == "Acme"
		}},
		{name: "Jobicy", typ: domain.Jobicy, body: `{"count":1,"jobs":[{"id":456,"jobTitle":"Backend Engineer","companyName":"Example","url":"https://jobicy.com/jobs/456","jobDescription":"Go and PostgreSQL","jobGeo":"Anywhere","jobType":["Full Time","Contract"],"pubDate":"2026-09-28T12:00:00Z"}]}`, check: func(j domain.ExternalJob) bool {
			return j.ExternalID == "456" && j.EmploymentType == "Full Time, Contract" && j.Location == "Anywhere"
		}},
		{name: "We Work Remotely", typ: domain.WeWorkRemotely, body: `<rss version="2.0"><channel><title>Jobs</title><item><title>Acme: Senior Go Engineer</title><region>Anywhere in the World</region><country></country><type>Full-Time</type><description>&lt;p&gt;Build Go services&lt;/p&gt;</description><pubDate>Mon, 28 Sep 2026 12:00:00 +0000</pubDate><guid>https://weworkremotely.com/remote-jobs/acme-senior-go-engineer</guid><link>https://weworkremotely.com/remote-jobs/acme-senior-go-engineer</link></item></channel></rss>`, check: func(j domain.ExternalJob) bool {
			return j.CompanyName == "Acme" && j.Title == "Senior Go Engineer" && j.EmploymentType == "Full-Time"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := NewRegistry()
			r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("User-Agent") == "" {
					t.Fatal("request omitted identifiable User-Agent")
				}
				return response(test.body), nil
			})
			got, err := r.Fetch(context.Background(), domain.Source{Type: test.typ})
			if err != nil {
				t.Fatal(err)
			}
			if got.Mode == domain.SyncSnapshot || len(got.Jobs) != 1 || !test.check(got.Jobs[0]) || got.Jobs[0].PublishedAt == nil {
				t.Fatalf("unexpected rolling feed result: %+v", got)
			}
		})
	}
}

func response(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func ExampleRegistry_Fetch() {
	var fetcher Fetcher = NewRegistry()
	fmt.Printf("registered: %t\n", fetcher != nil)
	// Output: registered: true
}
