package cmd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/healthcheck"
	"github.com/SanteonNL/orca/orchestrator/user"
)

func Start(config Config) error {
	if config.Validate() != nil {
		return fmt.Errorf("invalid configuration: %w", config.Validate())
	}

	// Set up dependencies
	httpHandler := http.NewServeMux()
	sessionManager := user.NewSessionManager()

	if err := config.Validate(); err != nil {
		return err
	}

	// Register services
	var services []Service

	services = append(services, healthcheck.New())

	var activeProfile profile.Provider = nuts.DutchNutsProfile{
		Config: config.Nuts,
	}
	if config.CarePlanContributor.Enabled {
		carePlanContributor, err := careplancontributor.New(
			config.CarePlanContributor,
			activeProfile,
			config.Public.ParseURL(),
			sessionManager)
		if err != nil {
			return err
		}
		services = append(services, carePlanContributor)
		// App Launches
		services = append(services, smartonfhir.New(config.CarePlanContributor.AppLaunch.SmartOnFhir, sessionManager, careplancontributor.LandingURL))
		if config.CarePlanContributor.AppLaunch.Demo.Enabled {
			services = append(services, demo.New(sessionManager, config.CarePlanContributor.AppLaunch.Demo, config.Public.URL, careplancontributor.LandingURL))
		}
		if config.CarePlanContributor.AppLaunch.ZorgPlatform.Enabled {
			services = append(services, zorgplatform.New(sessionManager, config.CarePlanContributor.AppLaunch.ZorgPlatform, config.Public.URL, careplancontributor.LandingURL))
		}
	}
	if config.CarePlanService.Enabled {
		carePlanService, err := careplanservice.New(config.CarePlanService, activeProfile, config.Public.ParseURL())
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
