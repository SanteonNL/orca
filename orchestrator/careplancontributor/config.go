package careplancontributor

import (
	"errors"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/dataview"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
)

func DefaultConfig() Config {
	return Config{
		Enabled:        true,
		AppLaunch:      applaunch.DefaultConfig(),
		SessionTimeout: 15 * time.Minute,
		FrontendConfig: FrontendConfig{
			URL: "/frontend/enrollment",
		},
	}
}

type Config struct {
	FrontendConfig FrontendConfig   `koanf:"frontend"`
	AppLaunch      applaunch.Config `koanf:"applaunch"`
	DataView       dataview.Config  `koanf:"dataview"`
	// FHIR contains the configuration to connect to the FHIR API holding EHR data,
	// to be made available through the CarePlanContributor.
	FHIR                          coolfhir.ClientConfig `koanf:"fhir"`
	TaskFiller                    TaskFillerConfig      `koanf:"taskfiller"`
	Enabled                       bool                  `koanf:"enabled"`
	HealthDataViewEndpointEnabled bool                  `koanf:"healthdataviewendpointenabled"`
	SessionTimeout                time.Duration         `koanf:"sessiontimeout"`
	Events                        EventsConfig          `koanf:"events"`
	StaticBearerToken             string
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	return nil
}

type TaskFillerConfig struct {
	// QuestionnaireFHIR contains the configuration to connect to the FHIR API holding Questionnaires and HealthcareServices,
	// used to negotiate FHIR Tasks. It might be a different FHIR API than the one holding EHR data,
	// also because HAPI doesn't allow storing Questionnaires in partitions.
	QuestionnaireFHIR     coolfhir.ClientConfig `koanf:"questionnairefhir"`
	QuestionnaireSyncURLs []string              `koanf:"questionnairesyncurls"`
	// TaskAcceptedBundleTopic is a Message Broker topic to which the TaskFiller will publish a message when a Task is accepted.
	// The bundle will contain the Task, Patient, and other relevant resources.
	TaskAcceptedBundleTopic string `koanf:"taskacceptedbundletopic"`
}

func (c TaskFillerConfig) Validate() error {
	for _, u := range c.QuestionnaireSyncURLs {
		if !strings.HasPrefix(u, "http://") &&
			!strings.HasPrefix(u, "https://") &&
			!strings.HasPrefix(u, "file://") {
			return errors.New("questionnairesyncurls must be http, https or file URLs")
		}
	}
	return nil
}

type EventsConfig struct {
	WebHooks []WebHookEventHandlerConfig `koanf:"webhooks"`
}

type WebHookEventHandlerConfig struct {
	// ResourceType is the FHIR resource type for which the event handler is triggered.
	// If blank, it's triggered for all resource types.
	ResourceType string
	// Name is the unique name of the event handler, used for persisting retry state.
	Name string
	// URL is the URL to which the event should be sent.
	URL string
}

type FrontendConfig struct {
	// URL is the base URL of the frontend for ORCA
	URL string `koanf:"url"`
}
