package cmd

import (
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/user"
	"net/http"
)

func Start(config Config) error {
	// Set up dependencies
	httpHandler := http.NewServeMux()
	didResolver := addressing.StaticDIDResolver(config.ParseURAMap())
	sessionManager := user.NewSessionManager()

	// Register services
	var services []Service
	if config.CarePlanContributor.Enabled {
		carePlanContributor, err := careplancontributor.New(config.CarePlanContributor, sessionManager, didResolver)
		if err != nil {
			return fmt.Errorf("failed to create CarePlanContributor: %w", err)
		}
		services = append(services, carePlanContributor)
		// App Launches
		services = append(services, smartonfhir.New(config.AppLaunch.SmartOnFhir, sessionManager))
		if config.AppLaunch.Demo.Enabled {
			services = append(services, demo.New(sessionManager, config.AppLaunch.Demo, config.Public.BaseURL))
		}
	}
	if config.CarePlanService.Enabled {
		carePlanService, err := careplanservice.New(config.CarePlanService, didResolver)
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
