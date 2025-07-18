package coolfhir

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"testing"
	"time"
)

func TestCreateSubscriptionNotification(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com/fhir")
	timestamp := time.Date(2024, 11, 1, 13, 44, 17, 0, time.UTC)
	subscription := fhir.Reference{Reference: to.Ptr("Subscription/123")}
	eventNumber := 1
	focus := fhir.Reference{Reference: to.Ptr("Patient/456")}
	expectedID := "d262df02-dc9b-4b30-80d4-3e4e99ddc691"
	expectedEntryID := "f2edf4be-140f-4ac8-b064-19af1ef6690e"
	expected := `{
  "id": "d262df02-dc9b-4b30-80d4-3e4e99ddc691",
  "meta": {
    "profile": [
      "http://hl7.org/fhir/uv/subscriptions-backport/StructureDefinition/backport-subscription-notification-r4"
    ]
  },
  "type": "history",
  "timestamp": "2024-11-01T13:44:17Z",
  "entry": [
    {
      "fullUrl": "urn:uuid:f2edf4be-140f-4ac8-b064-19af1ef6690e",
      "resource": {
        "id": "f2edf4be-140f-4ac8-b064-19af1ef6690e",
        "meta": {
          "profile": [
            "http://hl7.org/fhir/uv/subscriptions-backport/StructureDefinition/backport-subscription-notification-r4"
          ]
        },
        "parameter": [
          {
            "name": "subscription",
            "valueReference": {
              "reference": "Subscription/123"
            }
          },
          {
            "name": "status",
            "valueCode": "active"
          },
          {
            "name": "type",
            "valueString": "event-notification"
          },
          {
            "name": "notification-event",
            "part": [
              {
                "name": "event-number",
                "valueString": "1"
              },
              {
                "name": "timestamp",
                "valueInstant": "2024-11-01T13:44:17Z"
              },
              {
                "name": "focus",
                "valueReference": {
                  "reference": "https://example.com/fhir/Patient/456"
                }
              }
            ]
          }
        ],
        "resourceType": "Parameters"
      },
      "request": {
        "method": "GET",
        "url": "https://example.com/fhir/Subscription/123/$status"
      },
      "response": {
        "status": "200 OK"
      }
    }
  ]
}`

	notification := createSubscriptionNotification(baseURL, timestamp, subscription, eventNumber, focus, expectedID, expectedEntryID)

	actual, _ := json.MarshalIndent(notification, "", "  ")
	assert.JSONEq(t, expected, string(actual))
}

func TestIsSubscriptionNotification(t *testing.T) {
	assert.True(t, IsSubscriptionNotification(&fhir.Bundle{
		Type: fhir.BundleTypeHistory,
		Meta: &fhir.Meta{
			Profile: []string{"http://hl7.org/fhir/uv/subscriptions-backport/StructureDefinition/backport-subscription-notification-r4"},
		},
	}))
	assert.False(t, IsSubscriptionNotification(&fhir.Bundle{
		Type: fhir.BundleTypeHistory,
		Meta: &fhir.Meta{
			Profile: []string{"nope"},
		},
	}))
	assert.False(t, IsSubscriptionNotification(&fhir.Bundle{
		Type: fhir.BundleTypeHistory,
	}))
	assert.False(t, IsSubscriptionNotification(&fhir.Bundle{}))
}
