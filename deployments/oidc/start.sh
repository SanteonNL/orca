#!/bin/bash

set -e


# create a devtunnel, which stores its data in $1/data/devtunnel for re-usage over restarts
# It takes a port to forward to the tunnel, and returns the tunnel URL
# It kills the tunnel when this bash script exits
function createTunnel() {
  rm -f $1/data/tunnel.*
  local tunnelFile=$1/data/tunnel.id
  local tunnelPidFile=$1/data/tunnel.pid
  local tunnelPort=$2
  local tunnelLogFile=$1/data/tunnel.log
  # read from $1/data/tunnel.id
  # if it exists add it to the end of the devtunnel host command
  local devtunnel_host_command="devtunnel host -p ${tunnelPort} -a"
  if [ -f $tunnelFile ]; then
    devtunnel_host_command="devtunnel host $(cat $tunnelFile)"
  fi

  # Execute the devtunnel host command and write the output to a log file
  ${devtunnel_host_command} > $tunnelLogFile 2>&1 &
  # safe the pid for later
  echo $! > $tunnelPidFile

  # Try 30 times to read the tunnel URL from the log file.
  # The format is "your url is:" followed by the URL.
  local TUNNEL_URL=""
  local TUNNEL_ID=""
  for i in $(seq 1 10); do
    TUNNEL_URL=$(grep "Connect via browser:" $tunnelLogFile | awk '{print $5}')
    TUNNEL_ID=$(grep "Ready to accept connections for tunnel:" $tunnelLogFile | awk '{print $7}')
    if [ -n "$TUNNEL_URL" ]; then
      break
    fi
    sleep 1
  done

  # Check whether we retrieved the URL and if not, exit with an error.
  if [ -z "$TUNNEL_URL" ]; then
    >&2 echo "Failed to retrieve the devtunnel URL"
    >&2 cat $tunnelLogFile
    exit 1
  fi

  # store the tunnel id
  echo $TUNNEL_ID > $tunnelFile

  echo $TUNNEL_URL
}

export URL=$(createTunnel ./ 8080)
echo "Tunnel URL: $URL"
echo URL=$URL > data/.env

docker compose --env-file data/.env up --build --wait

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
open "${URL}/orca/demo-app-launch?practitioner=Practitioner%2F${PRACTITIONER_ID}&patient=Patient%2F1&serviceRequest=ServiceRequest%2F1&iss=${FHIR_BASEURL_ENCODED}"