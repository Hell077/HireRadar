package bootstrap

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRegisterDefaultsIsIdempotentAndPreservesOperatorSettings(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a PostgreSQL test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE sources (
		id text PRIMARY KEY, name text NOT NULL, source_type text NOT NULL,
		company_name text NOT NULL, config jsonb NOT NULL DEFAULT '{}'::jsonb,
		enabled boolean NOT NULL DEFAULT true, priority smallint NOT NULL DEFAULT 100,
		sync_interval_seconds integer NOT NULL DEFAULT 900)`); err != nil {
		t.Fatal(err)
	}
	inserted, err := RegisterDefaults(ctx, tx)
	if err != nil || inserted != len(Defaults) {
		t.Fatalf("first bootstrap inserted=%d error=%v", inserted, err)
	}
	var total int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM sources`).Scan(&total); err != nil || total != len(Defaults) {
		t.Fatalf("after first bootstrap count=%d error=%v", total, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE sources SET enabled=false,priority=17,sync_interval_seconds=7200 WHERE id=$1`, Defaults[0].ID); err != nil {
		t.Fatal(err)
	}
	inserted, err = RegisterDefaults(ctx, tx)
	if err != nil || inserted != 0 {
		t.Fatalf("second bootstrap inserted=%d error=%v", inserted, err)
	}
	var enabled bool
	var priority, interval int
	if err := tx.QueryRow(ctx, `SELECT enabled,priority,sync_interval_seconds FROM sources WHERE id=$1`, Defaults[0].ID).Scan(&enabled, &priority, &interval); err != nil {
		t.Fatal(err)
	}
	if enabled || priority != 17 || interval != 7200 {
		t.Fatalf("operator settings overwritten: enabled=%v priority=%d interval=%d", enabled, priority, interval)
	}
}
