package main

import (
	"errors"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/rs/zerolog/log"
	"net/http"
)

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}
	log.Info().Msgf("Public interface listens on %s", config.Public.Address)
	log.Info().Msgf("Using Nuts API on %s", config.Nuts.API.Address)

	httpHandler := http.NewServeMux()
	didResolver := addressing.StaticDIDResolver(config.URAMap)
	careplanservice.Service{
		DIDResolver: didResolver,
	}.RegisterHandlers(httpHandler)
	err = http.ListenAndServe(config.Public.Address, httpHandler)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
	log.Info().Msg("Goodbye!")
}
