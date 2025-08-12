//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"bytes"
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

var _ events.Type = &TaskAcceptedEvent{}

type TaskAcceptedEvent struct {
	FHIRBaseURL string    `json:"fhirBaseURL"`
	Task        fhir.Task `json:"task"`
	TenantID    string    `json:"tenantId"`
}

func (t TaskAcceptedEvent) Entity() messaging.Entity {
	return messaging.Entity{
		Name:   "orca.taskengine.task-accepted",
		Prefix: true,
	}
}

func (t TaskAcceptedEvent) Instance() events.Type {
	return &TaskAcceptedEvent{}
}

// Notifier is an interface for sending notifications regarding task acceptance within a FHIR-based system.
type Notifier interface {
	NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error
}

// notifier is a type that uses a ServiceBusClient to send messages to a message broker.
type notifier struct {
	tenants           tenants.Config
	eventManager      events.Manager
	fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)
}

// NewNotifier creates and returns a Notifier implementation using the provided ServiceBusClient for message handling.
func NewNotifier(eventManager events.Manager, tenants tenants.Config, taskAcceptedBundleEndpoint string, fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)) (Notifier, error) {
	n := &notifier{
		eventManager:      eventManager,
		fhirClientFactory: fhirClientFactory,
		tenants:           tenants,
	}
	if err := n.start(taskAcceptedBundleEndpoint); err != nil {
		return nil, err
	}
	return n, nil
}

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a message broker.
func (n *notifier) NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error {
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return err
	}
	return n.eventManager.Notify(ctx, TaskAcceptedEvent{
		TenantID:    tenant.ID,
		FHIRBaseURL: fhirBaseURL,
		Task:        *task,
	})
}

func (n *notifier) start(taskAcceptedBundleEndpoint string) error {
	// TODO: add sync call here to the API of DataHub for enrolling a patient
	// IF the call fails here we want to create a subtask to let the hospital know that the enrollment failed
	return n.eventManager.Subscribe(TaskAcceptedEvent{}, func(ctx context.Context, rawEvent events.Type) error {
		event := rawEvent.(*TaskAcceptedEvent)

		// Lookup tenant
		tenant, err := n.tenants.Get(event.TenantID)
		if err != nil {
			return errors.Wrapf(err, "failed to get from task accepted event (tenant-id=%s)", event.TenantID)
		}
		ctx = tenants.WithTenant(ctx, *tenant)

		fhirBaseURL, err := url.Parse(event.FHIRBaseURL)
		if err != nil {
			return err
		}

		cpsClient, _, err := n.fhirClientFactory(ctx, fhirBaseURL)
		if err != nil {
			return errors.Wrap(err, "failed to create FHIR client for invoking CarePlanService")
		}

		bundles, err := TaskNotificationBundleSet(ctx, cpsClient, *event.Task.Id)
		if err != nil {
			return errors.Wrap(err, "failed to create task notification bundle")
		}
		log.Ctx(ctx).Info().Msgf("Sending set for task notifier started")

		err = sendBundle(ctx, taskAcceptedBundleEndpoint, *bundles)
		if err != nil {
			var badRequest *BadRequest
			if errors.As(err, &badRequest) {
				// Handle BadRequest error specifically
				log.Ctx(ctx).Warn().Err(err).Msg("Task enrollment failed due to bad request")
				task := event.Task
				task.Status = fhir.TaskStatusRejected
				err = cpsClient.UpdateWithContext(ctx, "Task", event.Task.Id, task)
				if err != nil {
					return errors.Wrap(err, "failed to update task status")
				}
				return nil
			}
			return err
		}
		return nil
	})
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, taskAcceptedBundleEndpoint string, set BundleSet) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to HTTP endpoint failed (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)
	httpResponse, err := http.Post(taskAcceptedBundleEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Sending set for task (ref=%s) to HTTP endpoint failed (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)
		return errors.Wrap(err, "failed to send task to endpoint")
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusBadRequest {
		var badRequest BadRequest
		var operationOutcome fhir.OperationOutcome
		if err := json.NewDecoder(httpResponse.Body).Decode(&operationOutcome); err != nil {
			return errors.Wrap(err, "failed to decode bad request response")
		}
		if len(operationOutcome.Issue) > 0 {
			badRequest.Reason = operationOutcome.Issue[0].Diagnostics
		} else {
			unknownError := "unknown error"
			badRequest.Reason = &unknownError
		}
		return &BadRequest{Reason: badRequest.Reason}
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return errors.Errorf("failed to send task to endpoint, status code: %d", httpResponse.StatusCode)
	}
	log.Ctx(ctx).Info().Msgf("Notified EHR of accepted Task with bundle (ref=%s)", set.task)
	return nil
}

type BadRequest struct {
	Reason *string
}

func (e *BadRequest) Error() string {
	if e.Reason != nil {
		return *e.Reason
	}
	return ""
}
