package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	"github.com/SanteonNL/orca/orchestrator/healthcheck"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
)

func Start(config Config) error {
	if config.Validate() != nil {
		return fmt.Errorf("invalid configuration: %w", config.Validate())
	}

	// Set up dependencies
	httpHandler := http.NewServeMux()
	sessionManager := user.NewSessionManager(config.CarePlanContributor.SessionTimeout)

	if err := config.Validate(); err != nil {
		return err
	}

	// Register services
	var services []Service

	services = append(services, healthcheck.New())

	activeProfile, err := nuts.New(config.Nuts)
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}
	if config.CarePlanContributor.Enabled {
		// App Launches
		var ehrFhirProxy coolfhir.HttpProxy
		services = append(services, smartonfhir.New(config.CarePlanContributor.AppLaunch.SmartOnFhir, sessionManager, config.CarePlanContributor.FrontendConfig.URL))
		if config.CarePlanContributor.AppLaunch.Demo.Enabled {
			services = append(services, demo.New(sessionManager, config.CarePlanContributor.AppLaunch.Demo, config.CarePlanContributor.FrontendConfig.URL))
		}
		if config.CarePlanContributor.AppLaunch.ZorgPlatform.Enabled {
			service, err := zorgplatform.New(sessionManager, config.CarePlanContributor.AppLaunch.ZorgPlatform, config.Public.URL, config.CarePlanContributor.FrontendConfig.URL, activeProfile)
			if err != nil {
				return fmt.Errorf("failed to create Zorgplatform AppLaunch service: %w", err)
			}
			ehrFhirProxy = service.EhrFhirProxy()
			services = append(services, service)
		}
		//
		carePlanContributor, err := careplancontributor.New(
			config.CarePlanContributor,
			activeProfile,
			config.Public.ParseURL(),
			sessionManager,
			ehrFhirProxy,
			httpHandler)
		if err != nil {
			return err
		}
		services = append(services, carePlanContributor)

		// Start session expiration ticker
		ticker := time.NewTicker(time.Minute)
		go func() {
			for range ticker.C {
				sessionManager.PruneSessions()
			}
		}()
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
	err = http.ListenAndServe(config.Public.Address, httpHandler)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

type Service interface {
	RegisterHandlers(mux *http.ServeMux)
}
