#!/bin/bash

set -e

docker compose up --build --wait

FHIR_BASEURL="http://localhost:5826/fhir/r4"
FHIR_BASEURL_DOCKER="http://fhirstore:5826/fhir/r4"
FHIR_BASEURL_ENCODED=$(jq -rn --arg x "${FHIR_BASEURL_DOCKER}" '$x|@uri')

# Create Practitioner
PRACTITIONER_JSON=$(
cat << EOF
{
  "identifier": [
    {
      "system": "example.com/identifier",
      "value": "12345"
    }
  ],
  "name": [
    {
      "text": "John Doe"
    }
  ],
  "telecom": [
    {
      "system": "email",
      "value": "john@example.com"
    }
  ],
  "qualification": [
    {
      "code": {
        "coding": [
          {
            "system": "example.com/CodeSystem",
            "code": "nurse-level-4"
          }
        ]
      }
    }
  ],
  "resourceType": "Practitioner"
}
EOF
)

RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "${PRACTITIONER_JSON}" "$FHIR_BASEURL/Practitioner")
PRACTITIONER_ID=$(echo $RESPONSE | jq -r .id)

# %2F is the URL encoded version of /
open "http://localhost:8080/demo-app-launch?practitioner=Practitioner%2F${PRACTITIONER_ID}&patient=Patient%2F1&serviceRequest=ServiceRequest%2F1&iss=${FHIR_BASEURL_ENCODED}"