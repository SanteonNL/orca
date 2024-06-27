#!/bin/sh

echo "    Fetching $1/fhir/metadata"
# Wait for FHIR server to be ready
until $(curl --output /dev/null --silent --fail $1/fhir/metadata); do
  echo "    Waiting for FHIR server to be ready..."
  sleep 5
done

echo "    Creating SearchParameter for Task.reasonCode"
curl --silent -X PUT "$1/fhir/SearchParameter/task-reasonCode" \
    -H "Content-Type: application/fhir+json" \
    -d '{
        "resourceType": "SearchParameter",
        "id": "task-reasonCode",
        "url": "http://example.com/fhir/SearchParameter/task-reasonCode",
        "version": "1.0",
        "name": "reasonCode",
        "status": "active",
        "publisher": "Zorg Bij Jou",
        "contact": [
          {
            "name": "Support",
            "telecom": [
              {
                "system": "email",
                "value": "support@zorgbijjou.nl"
              }
            ]
          }
        ],
        "description": "Search by reason code",
        "code": "reasonCode",
        "base": ["Task"],
        "type": "token",
        "expression": "Task.reasonCode",
        "xpath": "f:Task/f:reasonCode",
        "xpathUsage": "normal"
    }'

# echo "    Creating Subscription for new telemonitoring requests" 
# curl -X PUT "$1/fhir/Subscription/task-telemonitoring-subscription" \
#     -H "Content-Type: application/fhir+json" \
#     -d '{
#         "resourceType": "Subscription",
#         "id": "task-telemonitoring-subscription",
#         "status": "active",
#         "criteria": "Task?status=requested",
#         "channel": {
#           "type": "rest-hook",
#           "endpoint": "'$1/viewer/api/task/new/telemonitoring'",
#           "payload": "application/fhir+json"
#         },
#         "reason": "Subscribe to telemonitoring tasks"
#     }'
