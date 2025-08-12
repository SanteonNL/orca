package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/events"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/ehr"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/subscriptions"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/healthcheck"
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
	sessionManager := user.NewSessionManager[session.Data](config.CarePlanContributor.SessionTimeout)

	// Set up tenant config
	for id, props := range config.Tenants {
		props.ID = id
		config.Tenants[id] = props
	}

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

	activeProfile, err := nuts.New(config.Nuts, config.Tenants)
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	orcaBaseURL := config.Public.ParseURL()
	if config.CarePlanContributor.Enabled {
		carePlanContributor, err := careplancontributor.New(
			config.CarePlanContributor,
			config.Tenants,
			activeProfile,
			orcaBaseURL,
			sessionManager,
			eventManager,
			config.CarePlanService.Enabled, httpHandler)
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
		carePlanService, err := careplanservice.New(config.CarePlanService, config.Tenants, activeProfile, orcaBaseURL, messageBroker, eventManager)
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
