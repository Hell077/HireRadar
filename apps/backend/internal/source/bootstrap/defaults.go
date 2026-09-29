// Package bootstrap registers the reviewed public ATS boards that ship with HireRadar.
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type Execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type DefaultSource struct {
	ID, Name, Type, Company, Board string
	IntervalSeconds                int
}

// Defaults contains only public boards whose current ATS endpoint and payload were verified.
var Defaults = []DefaultSource{
	{ID: "greenhouse-gitlab", Name: "GitLab jobs", Type: "greenhouse", Company: "GitLab", Board: "gitlab"},
	{ID: "greenhouse-cloudflare", Name: "Cloudflare jobs", Type: "greenhouse", Company: "Cloudflare", Board: "cloudflare"},
	{ID: "greenhouse-grafana-labs", Name: "Grafana Labs jobs", Type: "greenhouse", Company: "Grafana Labs", Board: "grafanalabs"},
	{ID: "greenhouse-mongodb", Name: "MongoDB jobs", Type: "greenhouse", Company: "MongoDB", Board: "mongodb"},
	{ID: "greenhouse-stripe", Name: "Stripe jobs", Type: "greenhouse", Company: "Stripe", Board: "stripe"},
	{ID: "greenhouse-coinbase", Name: "Coinbase jobs", Type: "greenhouse", Company: "Coinbase", Board: "coinbase"},
	{ID: "greenhouse-remote", Name: "Remote jobs", Type: "greenhouse", Company: "Remote", Board: "remotecom"},
	{ID: "ashby-linear", Name: "Linear jobs", Type: "ashby", Company: "Linear", Board: "linear"},
	{ID: "ashby-supabase", Name: "Supabase jobs", Type: "ashby", Company: "Supabase", Board: "supabase"},
	{ID: "ashby-posthog", Name: "PostHog jobs", Type: "ashby", Company: "PostHog", Board: "posthog"},
	{ID: "ashby-railway", Name: "Railway jobs", Type: "ashby", Company: "Railway", Board: "railway"},
	{ID: "ashby-render", Name: "Render jobs", Type: "ashby", Company: "Render", Board: "render"},
	{ID: "remoteok-public", Name: "RemoteOK public jobs", Type: "remoteok", Company: "RemoteOK", IntervalSeconds: 3600},
	{ID: "jobicy-public", Name: "Jobicy remote jobs", Type: "jobicy", Company: "Jobicy", IntervalSeconds: 3600},
	{ID: "weworkremotely-public", Name: "We Work Remotely jobs", Type: "weworkremotely", Company: "We Work Remotely", IntervalSeconds: 3600},
}

// RegisterDefaults inserts missing default boards but never changes an existing row.
func RegisterDefaults(ctx context.Context, db Execer) (int, error) {
	inserted := 0
	for _, source := range Defaults {
		config, err := json.Marshal(map[string]string{"board": source.Board})
		if err != nil {
			return inserted, err
		}
		interval := source.IntervalSeconds
		if interval == 0 {
			interval = 900
		}
		tag, err := db.Exec(ctx, `INSERT INTO sources(id,name,source_type,company_name,config,sync_interval_seconds)
			VALUES($1,$2,$3,$4,$5::jsonb,$6) ON CONFLICT (id) DO NOTHING`,
			source.ID, source.Name, source.Type, source.Company, config, interval)
		if err != nil {
			return inserted, fmt.Errorf("register default source %s: %w", source.ID, err)
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}
