package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
)

func TestSnapshotLoadsDefaultBranchArchiveAndUsesETag(t *testing.T) {
	const sha = "0123456789012345678901234567890123456789"
	archive := testArchive(t, map[string]string{
		"src/companies/example.md": "---\ntitle: Example\nwebsite: https://example.org\ncareers_url: https://example.org/jobs\n---\n",
	})
	archiveRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing GitHub auth header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/repos/owner/repo":
			_, _ = w.Write([]byte(`{"default_branch":"trunk"}`))
		case "/repos/owner/repo/commits/trunk":
			if r.Header.Get("If-None-Match") == `"commit-etag"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"commit-etag"`)
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
		case "/repos/owner/repo/tarball/" + sha:
			archiveRequests++
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewWithHTTP(server.URL, "test-token", server.Client())
	source := domain.DiscoverySource{ID: "remoteintech", Owner: "owner", Repo: "repo", Parser: "remoteintech"}
	snapshot, err := client.Snapshot(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.NotChanged || snapshot.CommitSHA != sha || snapshot.ETag != `"commit-etag"` || len(snapshot.Targets) != 1 || snapshot.Targets[0].Name != "Example" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	source.ETag, source.LastCommitSHA = snapshot.ETag, snapshot.CommitSHA
	snapshot, err = client.Snapshot(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.NotChanged || archiveRequests != 1 {
		t.Fatalf("unchanged snapshot=%#v archive requests=%d", snapshot, archiveRequests)
	}
}

func TestRateLimitUsesRetryAfterAndDoesNotExposeToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	client := NewWithHTTP(server.URL, "private-token", server.Client())
	_, _, err := client.get(context.Background(), server.URL+"/repos", "")
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) || rateErr.RetryDelay(time.Now()) < 59*time.Minute || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("unexpected rate limit error: %v", err)
	}
}

func TestVerifyJobRepositoryRequiresEnabledIssuesAndJobLikeSample(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/openings/jobs":
			_, _ = w.Write([]byte(`{"private":false,"archived":false,"disabled":false,"has_issues":true,"pushed_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}`))
		case "/repos/openings/jobs/issues":
			_, _ = w.Write([]byte(`[{"title":"Hiring: Senior Go Engineer","body":"Remote role. Apply with your CV.","html_url":"https://github.com/openings/jobs/issues/1"},{"title":"Frontend Developer","body":"Requirements: React experience. Apply at the link.","html_url":"https://github.com/openings/jobs/issues/2"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewWithHTTP(server.URL, "", server.Client())
	verified, err := client.VerifyJobRepository(context.Background(), "openings", "jobs")
	if err != nil || !verified.Verified || len(verified.Evidence) < 3 {
		t.Fatalf("verification=%+v err=%v", verified, err)
	}
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	for name, contents := range files {
		body := []byte(contents)
		if err := writer.WriteHeader(&tar.Header{Name: "repo-commit/" + name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
