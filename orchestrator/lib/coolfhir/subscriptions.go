package coolfhir

import (
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// SubscriptionNotification implements a SubscriptionNotification on FHIR R4 through a backport profile (http://hl7.org/fhir/uv/subscriptions-backport/StructureDefinition/backport-subscription-notification-r4).
// It provides helper functions to access the contained data.
type SubscriptionNotification fhir.Bundle

func IsSubscriptionNotification(bundle *fhir.Bundle) bool {
	return bundle != nil &&
		bundle.Type == fhir.BundleTypeHistory &&
		bundle.Meta != nil &&
		slices.Contains(bundle.Meta.Profile, "http://hl7.org/fhir/uv/subscriptions-backport/StructureDefinition/backport-subscription-notification-r4")
}

func (s SubscriptionNotification) GetFocus() (*fhir.Reference, error) {
	var notificationParams fhir.Parameters
	if err := ResourceInBundle((*fhir.Bundle)(&s), EntryIsOfType("Parameters"), &notificationParams); err != nil {
		return nil, err
	}
	for _, notificationParam := range notificationParams.Parameter {
		if notificationParam.Name == "notification-event" {
			for _, eventParam := range notificationParam.Part {
				if eventParam.Name == "focus" {
					return eventParam.ValueReference, nil
				}
			}
		}
	}
	return nil, errors.New("invalid R4 SubscriptionNotification: no focus found")
}

// CreateSubscriptionNotification creates a SubscriptionNotification according to https://santeonnl.github.io/shared-care-planning/Bundle-notification-msc-01.json.html
func CreateSubscriptionNotification(baseURL *url.URL, timestamp time.Time, subscription fhir.Reference, eventNumber int, focus fhir.Reference) SubscriptionNotification {
	// call non-exported function to aid test assertion
	return createSubscriptionNotification(baseURL, timestamp, subscription, eventNumber, focus, uuid.NewString(), uuid.NewString())
}

func createSubscriptionNotification(baseURL *url.URL, timestamp time.Time, subscription fhir.Reference, eventNumber int,
	focus fhir.Reference, resourceId string, resourceEntryId string) SubscriptionNotification {
	// INT-396: The focus reference should be an absolute URL
	focus.Reference = to.Ptr(baseURL.JoinPath(*focus.Reference).String())
	meta := fhir.Meta{
		Profile: []string{"http://hl7.org/fhir/uv/subscriptions-backport/StructureDefinition/backport-subscription-notification-r4"},
	}
	params := fhir.Parameters{
		Id:   to.Ptr(resourceEntryId),
		Meta: &meta,
		Parameter: []fhir.ParametersParameter{
			{
				Name:           "subscription",
				ValueReference: &subscription,
			},
			{
				Name:      "status",
				ValueCode: to.Ptr("active"),
			},
			{
				Name:        "type",
				ValueString: to.Ptr("event-notification"),
			},
			{
				Name: "notification-event",
				Part: []fhir.ParametersParameter{
					{
						Name:        "event-number",
						ValueString: to.Ptr(strconv.Itoa(eventNumber)),
					},
					{
						Name:         "timestamp",
						ValueInstant: to.Ptr(timestamp.Format(time.RFC3339)),
					},
					{
						Name:           "focus",
						ValueReference: &focus,
					},
				},
			},
		},
	}
	parametersJSON, _ := json.Marshal(params)
	return SubscriptionNotification(fhir.Bundle{
		Id:        to.Ptr(resourceId),
		Meta:      &meta,
		Type:      fhir.BundleTypeHistory,
		Timestamp: to.Ptr(timestamp.Format(time.RFC3339)),
		Entry: []fhir.BundleEntry{
			{
				FullUrl:  to.Ptr("urn:uuid:" + *params.Id),
				Resource: parametersJSON,
				Request: &fhir.BundleEntryRequest{
					Method: fhir.HTTPVerbGET,
					Url:    baseURL.JoinPath(*subscription.Reference, "$status").String(),
				},
				Response: &fhir.BundleEntryResponse{
					Status: "200 OK",
				},
			},
		},
	})
}
