package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

func Up(ctx context.Context, db *sql.DB) error {
	// Hold a session-level lock while goose checks and applies migrations so
	// concurrent API replicas cannot race through startup migrations.
	const lockID int64 = 0x4869726552616461
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration lock connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", lockID)
	}()

	goose.SetBaseFS(files)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, ".")
}
