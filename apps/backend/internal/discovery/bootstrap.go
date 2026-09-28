package discovery

import (
	"context"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

type Execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

var DefaultSources = []domain.DiscoverySource{
	{ID: "remoteintech", Name: "Remote In Tech", Owner: "remoteintech", Repo: "remote-jobs", Parser: "remoteintech", IntervalSeconds: 86400},
	{ID: "established-remote", Name: "Established Remote", Owner: "yanirs", Repo: "established-remote", Parser: "established_remote", IntervalSeconds: 86400},
	{ID: "global-hiring", Name: "Global Hiring", Owner: "ceolinwill", Repo: "global-hiring", Parser: "global_hiring", IntervalSeconds: 86400},
	{ID: "awesome-remote-job", Name: "Awesome Remote Job", Owner: "lukasz-madon", Repo: "awesome-remote-job", Parser: "awesome_remote_job", IntervalSeconds: 86400},
	{ID: "european-remote", Name: "European Remote", Owner: "EuropeanRemote", Repo: "european-remote-software-companies", Parser: "european_remote", IntervalSeconds: 86400},
	{ID: "remote-by-default", Name: "Remote By Default", Owner: "RemoteByDefault", Repo: "remote-software-companies", Parser: "remote_by_default", IntervalSeconds: 86400},
	{ID: "remote-freelancer", Name: "The Remote Freelancer", Owner: "engineerapart", Repo: "TheRemoteFreelancer", Parser: "remote_freelancer", IntervalSeconds: 86400},
	{ID: "remote-developer-directory", Name: "Remote Developer Jobs Directory", Owner: "ugglr", Repo: "Remote-Developer-jobs-directory", Parser: "remote_developer_directory", IntervalSeconds: 86400},
	{ID: "github-issue-job-boards", Name: "Awesome GitHub Issues Job Boards", Owner: "openings-dev", Repo: "awesome-github-issues-job-boards", Parser: "github_issue_boards", IntervalSeconds: 86400},
}

func RegisterSources(ctx context.Context, db Execer) (int, error) {
	inserted := 0
	for _, source := range DefaultSources {
		tag, err := db.Exec(ctx, `INSERT INTO discovery_sources(id,name,repo_owner,repo_name,parser_key,interval_seconds)
			VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO NOTHING`,
			source.ID, source.Name, source.Owner, source.Repo, source.Parser, source.IntervalSeconds)
		if err != nil {
			return inserted, fmt.Errorf("register discovery source %s: %w", source.ID, err)
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}
