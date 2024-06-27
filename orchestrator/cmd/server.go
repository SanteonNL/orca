package cmd

import (
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/user"
	"net/http"
	"net/url"
)

func Start(config Config) error {
	// Set up dependencies
	httpHandler := http.NewServeMux()
	didResolver := addressing.StaticDIDResolver(config.ParseURAMap())
	sessionManager := user.NewSessionManager()
	if config.CarePlanContributor.CarePlanService.URL == "" {
		return errors.New("careplancontributor.careplanservice.url is not configured")
	}
	cpsURL, _ := url.Parse(config.CarePlanContributor.CarePlanService.URL)
	// TODO: Replace with client doing authentication
	carePlanServiceClient := fhirclient.New(cpsURL, http.DefaultClient)

	// Register services
	services := []Service{
		careplancontributor.Service{
			SessionManager:  sessionManager,
			CarePlanService: carePlanServiceClient,
		},
		smartonfhir.New(config.AppLaunch.SmartOnFhir, sessionManager),
	}
	if config.CarePlanService.Enabled {
		carePlanService, err := careplanservice.New(config.CarePlanService, didResolver)
		if err != nil {
			return fmt.Errorf("failed to create CarePlanService: %w", err)
		}
		services = append(services, carePlanService)
	}
	if config.AppLaunch.Demo.Enabled {
		services = append(services, demo.New(sessionManager, config.AppLaunch.Demo, config.Public.BaseURL))
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
