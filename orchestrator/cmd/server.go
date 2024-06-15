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
	services := []Service{
		careplanservice.New(didResolver),
		careplancontributor.New(sessionManager),
		smartonfhir.New(config.AppLaunchConfig.SmartOnFhir, sessionManager),
		demo.New(sessionManager),
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
