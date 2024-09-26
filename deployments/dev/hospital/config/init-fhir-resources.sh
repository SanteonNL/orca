#!/bin/sh

echo "    Starting liveness check by fetching $1/fhir/metadata"
# Wait for FHIR server to be ready
until $(curl --output /dev/null --silent --fail $1/fhir/metadata); do
  echo "    Waiting for FHIR server to be ready..."
  sleep 5
done
