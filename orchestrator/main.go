package main

import (
	"github.com/SanteonNL/orca/orchestrator/outbound"
	"github.com/SanteonNL/orca/orchestrator/outbound/api"
	"github.com/SanteonNL/orca/orchestrator/outbound/web"
	"github.com/rs/zerolog/log"
	"net/url"
	"os"
	"strings"
	"sync"
)

func main() {
	var (
		nutsAPIAddress   string
		apiListenAddress string
		webListenAddress string
		baseURL          string
		demoConfigFile   string
	)
	envVariables := map[string]*string{
		"ORCHESTRATOR_NUTS_API_ADDRESS":   &nutsAPIAddress,
		"ORCHESTRATOR_API_LISTEN_ADDRESS": &apiListenAddress,
		"ORCHESTRATOR_WEB_LISTEN_ADDRESS": &webListenAddress,
		"ORCHESTRATOR_BASE_URL":           &baseURL,
		"ORCHESTRATOR_DEMO_CONFIGFILE":    &demoConfigFile,
	}
	for name, ptr := range envVariables {
		value := os.Getenv(name)
		if value == "" {
			log.Fatal().Msgf("Missing environment variable: %s", name)
		}
		*ptr = value
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse base URL")
	}

	log.Info().Msgf("Listening on %s", apiListenAddress)
	log.Info().Msgf("Using Nuts API on %s", nutsAPIAddress)
	log.Info().Msgf("User-facing API on %s", baseURL)

	httpServices := &sync.WaitGroup{}
	// Internal-facing API service
	var didResolver outbound.DIDResolver
	if strings.HasPrefix(demoConfigFile, "{") {
		didResolver, err = outbound.StaticDIDResolverFromJSON(demoConfigFile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse JSON demo config")
		}
	} else {
		didResolver = outbound.FileDIDResolver{File: demoConfigFile}
	}
	exchangeManager := outbound.NewExchangeManager(parsedBaseURL, nutsAPIAddress, didResolver)
	internalFacingAPI := api.Service{ExchangeManager: exchangeManager}
	httpServices.Add(1)
	go func() {
		defer httpServices.Done()
		if err := internalFacingAPI.Start(apiListenAddress); err != nil {
			log.Fatal().Err(err).Msg("API service failed")
		}
	}()
	// User-facing web service
	userFacingWeb := web.Service{ExchangeManager: exchangeManager}
	httpServices.Add(1)
	go func() {
		defer httpServices.Done()
		if err := userFacingWeb.Start(webListenAddress, apiListenAddress); err != nil {
			log.Fatal().Err(err).Msg("Web service failed")
		}
	}()

	httpServices.Wait()
	log.Info().Msg("Goodbye!")
}
