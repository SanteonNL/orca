#!/bin/bash

source ~/.bashrc
set -e

# create a devtunnel, which stores its data in $1/data/devtunnel for re-usage over restarts
# It takes a port to forward to the tunnel, and returns the tunnel URL
# It kills the tunnel when this bash script exits
function createTunnel() {
  rm $1/data/tunnel.*
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
    TUNNEL_URL=$(grep "Connect via browser:" $tunnelLogFile | awk '{print $4}')
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

# Create a root did:web DID. It takes the Nuts node URL and Nuts internal API.
# It then converts the Nuts node URL to a rooted did:web DID, e.g. https://something -> did:web:something
# It then creates the DID at the Nuts node and returns the DID.
function createDID() {
  NUTS_URL=$1
  NUTS_INTERNAL_API=$2
  DIDDOC=$(curl -s -X POST -H "Content-Type:application/json" -d "{\"did\": \"did:web:$(echo $NUTS_URL | sed 's/https:\/\///')\"}" $NUTS_INTERNAL_API/internal/vdr/v2/did)
  DID=$(echo $DIDDOC | jq -r .id)
  echo $DID
}

echo "Creating stack for Hospital..."
echo "  Creating devtunnel"
HOSPITAL_URL=$(createTunnel ./hospital 9080)
echo "  Starting services"
pushd hospital
NUTS_URL="${HOSPITAL_URL}" docker compose up --wait --build
echo "  Creating DID document"
HOSPITAL_DID=$(createDID $HOSPITAL_URL http://localhost:9081)
echo "    Hospital DID: $HOSPITAL_DID"
echo "  Registering FHIR base URL in DID document"
curl -X POST -H "Content-Type: application/json" -d "{\"type\":\"fhir-api\",\"serviceEndpoint\":\"${HOSPITAL_URL}/fhir\"}" http://localhost:9081/internal/vdr/v2/did/${HOSPITAL_DID}/service
popd


echo "Creating stack for Clinic..."
echo "  Creating devtunnel"
CLINIC_URL=$(createTunnel ./clinic 7080)
echo "  Starting services"
pushd clinic
touch data/orchestrator-demo-config.json
CLINIC_URL="${CLINIC_URL}" \
  docker compose up --wait --build

echo "  Creating DID document"
CLINIC_DID=$(createDID $CLINIC_URL http://localhost:8081)
echo "    Clinic DID: $CLINIC_DID"
echo "  Writing hardcoded orchestrator demo config"
echo "{
  \"clinic\": \"$CLINIC_DID\",
  \"hospital\": \"$HOSPITAL_DID\"
}" > data/orchestrator-demo-config.json
popd

# open orchestrator demo app
open "${CLINIC_URL}/orchestrator/"

# stop Docker services if script exits
trap "docker compose -f clinic/docker-compose.yaml down" EXIT
trap "docker compose -f hospital/docker-compose.yaml down" EXIT
trap "killall devtunnel" EXIT

# tail log of docker compose of both clinic and hospital
docker compose -f clinic/docker-compose.yaml logs -f &
docker compose -f hospital/docker-compose.yaml logs -f


