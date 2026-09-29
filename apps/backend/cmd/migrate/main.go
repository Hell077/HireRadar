package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/Hell077/HireRadar/apps/backend/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err == nil {
		defer db.Close()
		err = migrations.Up(context.Background(), db)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "apply database migrations: %v\n", err)
		os.Exit(1)
	}
}
