package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/SanteonNL/orca/orchestrator/cmd"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
)

func main() {
	config, err := cmd.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}
	// Set global slog logger, with context handler
	h := &logging.ContextHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     config.LogLevel,
			AddSource: true,
		}),
	}
	slog.SetDefault(slog.New(h))
	slog.Info(fmt.Sprintf("Public interface listens on %s", config.Public.Address))
	slog.Info(fmt.Sprintf("Using Nuts API on %s", config.Nuts.API.URL))
	if err := cmd.Start(context.Background(), *config); err != nil {
		slog.Error("Failed to start server", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("Goodbye!")
}
