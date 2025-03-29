package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/ehr"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/subscriptions"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/healthcheck"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/SanteonNL/orca/orchestrator/user"
)

// Start starts the server with the given configuration. It blocks until the given context is cancelled.
func Start(ctx context.Context, config Config) error {
	if config.Validate() != nil {
		return fmt.Errorf("invalid configuration: %w", config.Validate())
	}

	globals.StrictMode = config.StrictMode
	if !globals.StrictMode {
		log.Warn().Msg("Strict mode is disabled, do not use in production")
	}

	// Set up dependencies
	httpHandler := http.NewServeMux()
	sessionManager := user.NewSessionManager(config.CarePlanContributor.SessionTimeout)

	if err := config.Validate(); err != nil {
		return err
	}

	// Initialize Message Broker.
	// Collect topics so the message broker implementation can do checks on start-up whether it can actually publish to them.
	// Otherwise, things only break later at runtime.
	var messagingEntities []messaging.Entity
	if config.CarePlanContributor.TaskFiller.TaskAcceptedBundleTopic != "" {
		messagingEntities = append(messagingEntities, messaging.Entity{
			Name: config.CarePlanContributor.TaskFiller.TaskAcceptedBundleTopic,
		}, ehr.TaskAcceptedEvent{}.Entity())
	}
	if len(config.CarePlanService.Events.WebHooks) > 0 {
		messagingEntities = append(messagingEntities, careplanservice.CarePlanCreatedEvent{}.Entity())
	}
	if config.CarePlanService.Enabled {
		messagingEntities = append(messagingEntities, subscriptions.SendNotificationQueue)
	}
	messageBroker, err := messaging.New(config.Messaging, messagingEntities)
	if err != nil {
		return fmt.Errorf("message broker initialization: %w", err)
	}
	eventManager := events.NewManager(messageBroker)

	// Register services
	var services []Service

	services = append(services, healthcheck.New())

	activeProfile, err := nuts.New(config.Nuts)
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}
	if config.CarePlanContributor.Enabled {
		// App Launches
		frontendUrl, _ := url.Parse(config.CarePlanContributor.FrontendConfig.URL)
		services = append(services, smartonfhir.New(config.CarePlanContributor.AppLaunch.SmartOnFhir, sessionManager, frontendUrl))

		var ehrFhirProxy coolfhir.HttpProxy //TODO: Rewrite to an array so we can support multiple login mechanisms and multiple EHR proxies
		if config.CarePlanContributor.AppLaunch.Demo.Enabled {
			services = append(services, demo.New(sessionManager, config.CarePlanContributor.AppLaunch.Demo, frontendUrl))
		}
		if config.CarePlanContributor.AppLaunch.ZorgPlatform.Enabled {
			service, err := zorgplatform.New(sessionManager, config.CarePlanContributor.AppLaunch.ZorgPlatform, config.Public.URL, frontendUrl, activeProfile)
			if err != nil {
				return fmt.Errorf("failed to create Zorgplatform AppLaunch service: %w", err)
			}
			ehrFhirProxy = service.EhrFhirProxy()
			services = append(services, service)
		}
		var cpsURL *url.URL
		if config.CarePlanService.Enabled {
			cpsURL = config.Public.ParseURL().JoinPath("cps")
		}
		carePlanContributor, err := careplancontributor.New(
			config.CarePlanContributor,
			activeProfile,
			config.Public.ParseURL(),
			sessionManager,
			messageBroker,
			eventManager,
			ehrFhirProxy,
			cpsURL)
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
		carePlanService, err := careplanservice.New(config.CarePlanService, activeProfile, config.Public.ParseURL(), messageBroker, eventManager)
		if err != nil {
			return fmt.Errorf("failed to create CarePlanService: %w", err)
		}
		services = append(services, carePlanService)
	}

	for _, service := range services {
		service.RegisterHandlers(httpHandler)
	}

	// Start HTTP server, shutdown when given context.Context is cancelled
	httpServer := &http.Server{Addr: config.Public.Address, Handler: httpHandler}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	listenChan := make(chan error)
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// couldn't start server
			listenChan <- err
		}
	}()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)

	select {
	case listenErr := <-listenChan:
		return fmt.Errorf("failed to start HTTP server: %w", listenErr)
	case <-interruptChan:
		// Interrupt signal, need to shut down gracefully
		break
	case <-ctx.Done():
		// Start context cancelled, need to shut down gracefully
		break
	}
	log.Info().Msg("Shutting down...")
	if err := httpServer.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("failed to shut down HTTP server: %w", err)
	}
	// Graceful shutdown
	return nil
}

type Service interface {
	RegisterHandlers(mux *http.ServeMux)
}
