package main

import (
	"log/slog"
	"os"

	"github.com/Hell077/HireRadar/apps/backend/internal/adapters/in/httpapi"
	"github.com/Hell077/HireRadar/apps/backend/internal/application/health"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := httpapi.New(health.NewService())
	if err := app.Listen(":" + port); err != nil {
		slog.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}
