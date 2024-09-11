package cmd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/healthcheck"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/nuts-foundation/go-nuts-client/nuts"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
)

func Start(config Config) error {
	if config.Validate() != nil {
		return fmt.Errorf("invalid configuration: %w", config.Validate())
	}

	// Set up dependencies
	httpHandler := http.NewServeMux()
	didResolver := addressing.StaticDIDResolver(map[string]string{})
	sessionManager := user.NewSessionManager()

	if err := config.Validate(); err != nil {
		return err
	}
	nutsOAuth2HttpClient := &http.Client{
		Transport: &oauth2.Transport{
			TokenSource: nuts.OAuth2TokenSource{
				OwnDID:     config.Nuts.OwnDID,
				NutsAPIURL: config.Nuts.API.URL,
			},
			MetadataLoader: &oauth2.MetadataLoader{},
			AuthzServerLocators: []oauth2.AuthorizationServerLocator{
				oauth2.ProtectedResourceMetadataLocator,
			},
			Scope: careplancontributor.CarePlanServiceOAuth2Scope,
		},
	}

	// Register services
	var services []Service

	services = append(services, healthcheck.New())

	if config.CarePlanContributor.Enabled {
		carePlanContributor, err := careplancontributor.New(
			config.CarePlanContributor,
			config.Nuts.Public.Parse(),
			config.Public.ParseURL(),
			config.Nuts.API.Parse(),
			config.Nuts.OwnDID,
			sessionManager,
			nutsOAuth2HttpClient,
			didResolver)
		if err != nil {
			return err
		}
		services = append(services, carePlanContributor)
		// App Launches
		services = append(services, smartonfhir.New(config.AppLaunch.SmartOnFhir, sessionManager))
		if config.AppLaunch.Demo.Enabled {
			services = append(services, demo.New(sessionManager, config.AppLaunch.Demo, config.Public.URL))
		}
	}
	if config.CarePlanService.Enabled {
		carePlanService, err := careplanservice.New(config.CarePlanService, config.Nuts.Public.Parse(),
			config.Public.ParseURL(), config.Nuts.API.Parse(), config.Nuts.OwnDID, didResolver)
		if err != nil {
			return fmt.Errorf("failed to create CarePlanService: %w", err)
		}
		services = append(services, carePlanService)
	}

	for _, service := range services {
		service.RegisterHandlers(httpHandler)
	}

	// Start HTTP server
	err := http.ListenAndServe(config.Public.Address, httpHandler)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

type Service interface {
	RegisterHandlers(mux *http.ServeMux)
}
