package main

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/cmd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	config, err := cmd.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}
	zerolog.SetGlobalLevel(config.LogLevel)
	log.Info().Msgf("Public interface listens on %s", config.Public.Address)
	log.Info().Msgf("Using Nuts API on %s", config.Nuts.API.URL)
	if err := cmd.Start(context.Background(), *config); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
	log.Info().Msg("Goodbye!")
}
