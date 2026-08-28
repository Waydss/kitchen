package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/app"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/platform/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.LoadAPI()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.RunAPI(ctx, cfg, logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
