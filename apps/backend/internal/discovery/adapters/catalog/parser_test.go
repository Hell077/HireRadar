package catalog

import (
	"testing"

	"github.com/Hell077/HireRadar/apps/backend/internal/discovery/domain"
)

func TestParseRemoteInTechCurrentFrontmatterFormat(t *testing.T) {
	targets, err := Parse("remoteintech", map[string][]byte{"src/companies/example.md": []byte(`---
title: "Example Co"
slug: example-co
website: https://www.example.com/
careers_url: https://example.com/careers
region: worldwide
remote_policy: fully-remote
technologies:
  - golang
  - postgres
---

## Company blurb
A company.
`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets=%d", len(targets))
	}
	target := targets[0]
	if target.Name != "Example Co" || target.ExternalKey != "example" || target.OfficialDomain != "example.com" || target.CareersURL != "https://example.com/careers" || len(target.Technologies) != 2 || target.Regions[0] != "worldwide" {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestRemoteInTechDoesNotUseSharedATSHostAsCompanyDomain(t *testing.T) {
	body := "---\ntitle: Example\ncareers_url: https://boards.greenhouse.io/example/jobs\n---\n"
	targets, err := Parse("remoteintech", map[string][]byte{"src/companies/example.md": []byte(body)})
	if err != nil || len(targets) != 1 || targets[0].OfficialDomain != "" {
		t.Fatalf("targets=%#v err=%v", targets, err)
	}
}

func TestParseEstablishedRemoteCompanyTableAndCareersLink(t *testing.T) {
	readme := `[10up](https://10up.com/) | Web consulting | WordPress, PHP, React | ✗ | :keyboard: [Jobs](https://10up.com/careers/)<br>:door: [Glassdoor](https://glassdoor.example/10up)
[Grafana Labs](http://www.grafana.com) | Monitoring & analytics | Go, Java, Rust | ✓ | :keyboard: [Jobs](https://grafana.com/about/careers/open-positions/)<br>:door: [Glassdoor](https://glassdoor.example/grafana)
`
	targets, err := Parse("established_remote", map[string][]byte{"README.md": []byte(readme)})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets=%d", len(targets))
	}
	if targets[1].Name != "Grafana Labs" || targets[1].CareersURL != "https://grafana.com/about/careers/open-positions/" || targets[1].OfficialDomain != "grafana.com" || targets[1].Technologies[0] != "Go" {
		t.Fatalf("unexpected Grafana Labs target: %#v", targets[1])
	}
}

func TestParseGlobalHiringRestrictionsAsMetadata(t *testing.T) {
	readme := `| Company | Business | Languages | Open Salary | Restrictions |
| --- | --- | --- | --- | --- |
| [Example](https://example.com/) | Infrastructure | Go, TypeScript | Yes | Some country restrictions |
`
	targets, err := Parse("global_hiring", map[string][]byte{"README.md": []byte(readme)})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Name != "Example" || targets[0].OfficialDomain != "example.com" || targets[0].CareersURL != "" {
		t.Fatalf("unexpected global hiring target: %#v", targets)
	}
	if string(targets[0].DiscoveryFields) == "" || !containsAny(string(targets[0].DiscoveryFields), "Some country restrictions") {
		t.Fatalf("restriction metadata was lost: %s", targets[0].DiscoveryFields)
	}
}

func TestParseRegionalRemoteCompanyDirectories(t *testing.T) {
	readme := `# European Remote
| Company | Domain | Tech-stack | Salary transparency | Global salary | Profile |
| ------- | ------ | ---------- | ------------------- | ------------- | ------- |
|1Password 🇨🇦 |Security|🖥 Golang 🎨 React, TypeScript ☁️ AWS| ❌ | ❌ |[ℹ️](https://europeanremote.com/teams/1password) |
`
	for _, parser := range []string{"european_remote", "remote_by_default"} {
		targets, err := Parse(parser, map[string][]byte{"README.md": []byte(readme)})
		if err != nil || len(targets) != 1 {
			t.Fatalf("%s targets=%d err=%v", parser, len(targets), err)
		}
		if targets[0].Name != "1Password" || targets[0].WebsiteURL != "" || len(targets[0].Technologies) != 4 {
			t.Fatalf("unexpected regional target: %#v", targets[0])
		}
	}
}

func TestParseAwesomeRemoteJobKeepsJobBoardsAndFreelancePlatforms(t *testing.T) {
	readme := `## Job boards
1. [RemoteOK](https://remoteok.io/) - Remote jobs.
## Freelance platforms
- [Upwork](https://upwork.com/) - Contract marketplace.
## Articles & Posts
- [Not a source](https://example.com/article)
`
	targets, err := Parse("awesome_remote_job", map[string][]byte{"README.md": []byte(readme)})
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets=%d err=%v", len(targets), err)
	}
	if targets[0].Kind != domain.JobBoardTarget || targets[1].Kind != domain.FreelancePlatformTarget {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestParseRemoteFreelancerSections(t *testing.T) {
	readme := `| **Clients** | | | |
| [Upwork](https://upwork.com) | 200 | | Developers |
| **Jobs** | | | |
| [Remote Jobs](https://jobs.example.com) | | | Developers |
`
	targets, err := Parse("remote_freelancer", map[string][]byte{"README.md": []byte(readme)})
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets=%d err=%v", len(targets), err)
	}
	if targets[0].Kind != domain.FreelancePlatformTarget || targets[1].Kind != domain.JobBoardTarget {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestParseRemoteDeveloperHTMLSections(t *testing.T) {
	readme := `<h2>Remote Jobs Directories</h2><ul><li>Remotive: https://remotive.io/</li></ul><h2>Freelancing</h2><ul><li>Upwork: https://upwork.com/</li></ul>`
	targets, err := Parse("remote_developer_directory", map[string][]byte{"README.md": []byte(readme)})
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets=%d err=%v", len(targets), err)
	}
	if targets[0].Kind != domain.JobBoardTarget || targets[1].Kind != domain.FreelancePlatformTarget {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestParseGitHubIssueBoardRepositoryLinks(t *testing.T) {
	readme := `# Global
- [pythonjobs/jobs](https://github.com/pythonjobs/jobs) - Python roles.
- [Not a repo](https://example.com/jobs)
`
	targets, err := Parse("github_issue_boards", map[string][]byte{"README.md": []byte(readme)})
	if err != nil || len(targets) != 1 || targets[0].ExternalKey != "pythonjobs/jobs" || targets[0].Kind != domain.GitHubJobsTarget {
		t.Fatalf("targets=%#v err=%v", targets, err)
	}
}
