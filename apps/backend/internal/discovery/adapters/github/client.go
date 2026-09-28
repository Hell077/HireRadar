package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/adapters/catalog"
	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	atsadapter "github.com/Hell077/HireRadar/apps/backend/internal/source/adapters/ats"
)

const (
	maxArchiveBytes   = 64 << 20
	maxExpandedBytes  = 128 << 20
	maxCatalogFiles   = 8000
	maxCatalogFile    = 1 << 20
	maxGitHubBodySize = 2 << 20
)

type Client struct {
	http          *http.Client
	apiURL        string
	archiveURL    string
	directArchive bool
	token         string
	allowHost     map[string]bool
}

type RateLimitError struct {
	RetryAt time.Time
	Status  int
}

// VerifyJobRepository confirms a public, non-archived GitHub repository has
// Issues enabled and current open Issues that resemble actual job postings.
func (c *Client) VerifyJobRepository(ctx context.Context, owner, repo string) (domain.GitHubRepositoryVerification, error) {
	owner, repo = strings.TrimSpace(owner), strings.TrimSpace(repo)
	if owner == "" || repo == "" || strings.ContainsAny(owner+repo, "/\\") || owner == "." || owner == ".." || repo == "." || repo == ".." {
		return domain.GitHubRepositoryVerification{Reason: "repository owner or name is invalid"}, nil
	}
	base := fmt.Sprintf("%s/repos/%s/%s", c.apiURL, url.PathEscape(owner), url.PathEscape(repo))
	body, _, err := c.get(ctx, base, "")
	if err != nil {
		return domain.GitHubRepositoryVerification{}, err
	}
	var metadata struct {
		Private   bool   `json:"private"`
		Archived  bool   `json:"archived"`
		Disabled  bool   `json:"disabled"`
		HasIssues bool   `json:"has_issues"`
		PushedAt  string `json:"pushed_at"`
	}
	if err := json.Unmarshal(body, &metadata); err != nil {
		return domain.GitHubRepositoryVerification{}, fmt.Errorf("decode GitHub job repository: %w", err)
	}
	if metadata.Private || metadata.Archived || metadata.Disabled || !metadata.HasIssues {
		return domain.GitHubRepositoryVerification{Reason: "repository must be public, active, and have Issues enabled"}, nil
	}
	if pushedAt, err := time.Parse(time.RFC3339, metadata.PushedAt); err != nil || time.Since(pushedAt) > 2*365*24*time.Hour {
		return domain.GitHubRepositoryVerification{Reason: "repository has no recent activity and needs manual review"}, nil
	}
	issuesURL := base + "/issues?state=open&per_page=5&page=1"
	issuesBody, _, err := c.get(ctx, issuesURL, "")
	if err != nil {
		return domain.GitHubRepositoryVerification{}, err
	}
	var issues []struct {
		Title string          `json:"title"`
		Body  string          `json:"body"`
		URL   string          `json:"html_url"`
		Pull  json.RawMessage `json:"pull_request"`
	}
	if err := json.Unmarshal(issuesBody, &issues); err != nil {
		return domain.GitHubRepositoryVerification{}, fmt.Errorf("decode GitHub job repository issues: %w", err)
	}
	if len(issues) == 0 {
		return domain.GitHubRepositoryVerification{Reason: "repository has no open job postings to verify"}, nil
	}
	jobIssues := 0
	evidence := []string{fmt.Sprintf("public repository %s/%s, Issues enabled", owner, repo)}
	for _, issue := range issues {
		if len(issue.Pull) > 0 {
			continue
		}
		if atsadapter.IsJobPosting(issue.Title, issue.Body) {
			jobIssues++
			evidence = append(evidence, "job-like issue: "+issue.Title)
		}
	}
	if jobIssues == 0 || jobIssues*2 < len(issues) {
		return domain.GitHubRepositoryVerification{Reason: "sample open Issues do not reliably resemble job postings"}, nil
	}
	evidence = append(evidence, fmt.Sprintf("%d of %d sampled open issues resemble job postings", jobIssues, len(issues)))
	return domain.GitHubRepositoryVerification{Verified: true, Evidence: evidence}, nil
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit response (HTTP %d), retry at %s", e.Status, e.RetryAt.UTC().Format(time.RFC3339))
}

func (e *RateLimitError) RetryDelay(now time.Time) time.Duration {
	d := e.RetryAt.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

func New(token string) *Client {
	return NewWithHTTP("https://api.github.com", token, &http.Client{Timeout: 45 * time.Second})
}

func NewWithHTTP(apiURL, token string, client *http.Client) *Client {
	parsed, _ := url.Parse(apiURL)
	apiHost := strings.ToLower(parsed.Hostname())
	archiveURL := strings.TrimRight(apiURL, "/")
	directArchive := apiHost == "api.github.com"
	if directArchive {
		archiveURL = "https://codeload.github.com"
	}
	copyClient := *client
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" || !validRedirectHost(req.URL.Hostname(), apiHost) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &Client{http: &copyClient, apiURL: strings.TrimRight(apiURL, "/"), archiveURL: archiveURL, directArchive: directArchive, token: token, allowHost: map[string]bool{apiHost: true, "codeload.github.com": true}}
}

func (c *Client) Snapshot(ctx context.Context, source domain.DiscoverySource) (domain.RepositorySnapshot, error) {
	owner := url.PathEscape(source.Owner)
	repo := url.PathEscape(source.Repo)
	// Resolve the repository's default branch rather than assuming main/master.
	repositoryURL := fmt.Sprintf("%s/repos/%s/%s", c.apiURL, owner, repo)
	repoBody, _, err := c.get(ctx, repositoryURL, "")
	if err != nil {
		return domain.RepositorySnapshot{}, err
	}
	var repository struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(repoBody, &repository); err != nil {
		return domain.RepositorySnapshot{}, fmt.Errorf("decode GitHub repository default branch: %w", err)
	}
	if repository.DefaultBranch == "" {
		return domain.RepositorySnapshot{}, errors.New("decode GitHub repository default branch: field is empty")
	}
	commitURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.apiURL, owner, repo, url.PathEscape(repository.DefaultBranch))
	commitBody, headers, err := c.get(ctx, commitURL, source.ETag)
	if errors.Is(err, errNotModified) {
		return domain.RepositorySnapshot{ETag: source.ETag, CommitSHA: source.LastCommitSHA, NotChanged: true}, nil
	}
	if err != nil {
		return domain.RepositorySnapshot{}, err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(commitBody, &commit); err != nil {
		return domain.RepositorySnapshot{}, fmt.Errorf("decode GitHub commit SHA: %w", err)
	}
	if len(commit.SHA) < 40 {
		return domain.RepositorySnapshot{}, errors.New("decode GitHub commit SHA: response did not contain a full commit SHA")
	}
	archiveURL := fmt.Sprintf("%s/repos/%s/%s/tarball/%s", c.archiveURL, owner, repo, url.PathEscape(commit.SHA))
	if c.directArchive {
		archiveURL = fmt.Sprintf("%s/%s/%s/tar.gz/%s", c.archiveURL, owner, repo, url.PathEscape(commit.SHA))
	}
	archive, _, err := c.get(ctx, archiveURL, "")
	if err != nil {
		return domain.RepositorySnapshot{}, err
	}
	files, err := readArchive(archive, source.Parser)
	if err != nil {
		return domain.RepositorySnapshot{}, err
	}
	targets, err := catalog.Parse(source.Parser, files)
	if err != nil {
		return domain.RepositorySnapshot{}, err
	}
	etag := headers.Get("ETag")
	if etag == "" {
		etag = `"` + commit.SHA + `"`
	}
	return domain.RepositorySnapshot{ETag: etag, CommitSHA: commit.SHA, Targets: targets}, nil
}

var errNotModified = errors.New("GitHub repository not modified")

func (c *Client) get(ctx context.Context, endpoint, etag string) ([]byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "HireRadar-source-discovery")
	if c.token != "" && c.allowHost[strings.ToLower(request.URL.Hostname())] && strings.ToLower(request.URL.Hostname()) != "codeload.github.com" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	var response *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		response, err = c.http.Do(request)
		if err != nil {
			return nil, nil, fmt.Errorf("request GitHub API: %w", err)
		}
		if response.StatusCode != http.StatusTooManyRequests && response.StatusCode != http.StatusForbidden {
			break
		}
		rateErr := rateLimit(response)
		_ = response.Body.Close()
		if attempt == 0 && rateErr.RetryDelay(time.Now()) <= 5*time.Second {
			timer := time.NewTimer(rateErr.RetryDelay(time.Now()))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		return nil, nil, rateErr
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return nil, response.Header.Clone(), errNotModified
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, nil, fmt.Errorf("GitHub API returned HTTP %d for %s", response.StatusCode, endpoint)
	}
	limit := int64(maxGitHubBodySize)
	if strings.Contains(endpoint, "/tarball/") || strings.Contains(endpoint, "/tar.gz/") {
		limit = maxArchiveBytes
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(body)) > limit {
		return nil, nil, fmt.Errorf("GitHub response exceeds %d bytes", limit)
	}
	return body, response.Header.Clone(), nil
}

func rateLimit(response *http.Response) *RateLimitError {
	now := time.Now()
	retryAt := now.Add(time.Minute)
	if value := response.Header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
			retryAt = now.Add(time.Duration(seconds) * time.Second)
		} else if parsed, err := http.ParseTime(value); err == nil {
			retryAt = parsed
		}
	} else if value := response.Header.Get("X-RateLimit-Reset"); value != "" && response.Header.Get("X-RateLimit-Remaining") == "0" {
		if epoch, err := strconv.ParseInt(value, 10, 64); err == nil {
			retryAt = time.Unix(epoch, 0)
		}
	}
	return &RateLimitError{RetryAt: retryAt, Status: response.StatusCode}
}

func readArchive(data []byte, parser string) (map[string][]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open GitHub repository archive: %w", err)
	}
	defer reader.Close()
	archive := tar.NewReader(reader)
	files := make(map[string][]byte)
	var expanded int64
	count := 0
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read GitHub repository archive: %w", err)
		}
		count++
		if count > maxCatalogFiles {
			return nil, fmt.Errorf("GitHub archive contains more than %d entries", maxCatalogFiles)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if header.Size < 0 {
			return nil, fmt.Errorf("GitHub archive entry %s has a negative size", header.Name)
		}
		expanded += header.Size
		if expanded > maxExpandedBytes {
			return nil, fmt.Errorf("GitHub archive expands beyond %d bytes", maxExpandedBytes)
		}
		name := strings.TrimPrefix(header.Name, "./")
		if index := strings.IndexByte(name, '/'); index >= 0 {
			name = name[index+1:]
		}
		if !wantedFile(parser, name) || header.Size > maxCatalogFile {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(archive, maxCatalogFile+1))
		if err != nil || int64(len(body)) != header.Size {
			return nil, fmt.Errorf("read catalog file %s: %w", name, err)
		}
		files[name] = body
	}
	return files, nil
}

func wantedFile(parser, name string) bool {
	switch parser {
	case "remoteintech":
		return strings.HasPrefix(name, "src/companies/") && strings.HasSuffix(name, ".md")
	case "established_remote", "global_hiring", "awesome_remote_job", "european_remote", "remote_by_default", "remote_freelancer", "remote_developer_directory", "github_issue_boards":
		return name == "README.md"
	default:
		return false
	}
}

func validRedirectHost(host, apiHost string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == apiHost || host == "codeload.github.com"
}
