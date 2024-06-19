package cmd

import (
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"net/http"
	"net/url"
)

func Start(config Config) error {
	// Set up dependencies
	httpHandler := http.NewServeMux()
	didResolver := addressing.StaticDIDResolver(config.ParseURAMap())
	sessionManager := user.NewSessionManager()
	if config.CarePlanService.URL == "" {
		return errors.New("careplanservice.url is not configured")
	}
	cpsURL, _ := url.Parse(config.CarePlanService.URL)
	// TODO: Replace with client doing authentication
	carePlanServiceClient := coolfhir.NewClient(cpsURL, http.DefaultClient)

	// Register services
	services := []Service{
		careplanservice.New(didResolver),
		careplancontributor.Service{
			SessionManager:  sessionManager,
			CarePlanService: carePlanServiceClient,
		},
		smartonfhir.New(config.AppLaunch.SmartOnFhir, sessionManager),
	}
	if config.AppLaunch.Demo.Enabled {
		services = append(services, demo.New(sessionManager))
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
