#!/bin/bash

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

# Create a root did:web DID. It takes the party name, Nuts node URL and Nuts internal API.
# It then converts the Nuts node URL to a rooted did:web DID, e.g. https://something -> did:web:something
# It then creates the DID at the Nuts node and returns the DID.
function createDID() {
  NAME=$1
  NUTS_INTERNAL_API=$2
  DIDDOC=$(curl -s -X POST -H "Content-Type:application/json" -d "{\"subject\": \"${NAME}\"}" $NUTS_INTERNAL_API/internal/vdr/v2/subject)
  DID=$(echo $DIDDOC | jq -r .documents[0].id)
  echo $DID
}

# Self-issue a UziServerCertificateCredential, backed by a UZI server certificate. It takes:
# - Subject ID holding the wallet the VC should be loaded into
# - The DID of the entity to issue the credential to
function issueUraCredential() {
  SUBJECT=$1
  DID=$2

  CRED=$(docker run --rm \
    -v "./uzi-certificate.pem:/uzi-certificate.pem:ro" \
    -v "./uzi-certificate-key.pem:/uzi-certificate-key.pem:ro" \
    reinkrul/uzi-did-x509-issuer:local \
    vc /uzi-certificate.pem /uzi-certificate-key.pem "${DID}")
  echo $CRED
  docker compose exec nutsnode curl -s -X POST -d "\"${CRED}\"" -H "Content-Type: application/json" "http://localhost:8081/internal/vcr/v2/holder/${SUBJECT}/vc"
}

echo "Creating stack for Hospital..."
echo "  Creating devtunnel"
export HOSPITAL_URL=$(createTunnel ./hospital 9080)
export CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL="${HOSPITAL_URL}/orca/cps"
echo "  Creating Discovery Service definition"
HOSPITAL_URL_ESCAPED=$(sed 's/[&/\]/\\&/g' <<<"${HOSPITAL_URL}")
sed "s/DiscoveryServerURL/${HOSPITAL_URL_ESCAPED}/" shared_config/discovery_input/homemonitoring.json > shared_config/discovery/homemonitoring.json
echo "  Starting services"
pushd hospital
echo NUTS_URL="${HOSPITAL_URL}" >> .env
echo CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL="${CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}" >> .env
#docker compose --env-file .env pull
docker compose --env-file .env up --wait --build --remove-orphans
echo "  Creating DID document"
HOSPITAL_DID=$(createDID "hospital" http://localhost:9081)
echo "    Hospital DID: $HOSPITAL_DID"
echo "  Self-issuing an NutsUraCredential"
issueUraCredential "hospital" "${HOSPITAL_DID}" "4567" "Demo Hospital" "Amsterdam"
echo "  Registering on Nuts Discovery Service"
curl -X POST -H "Content-Type: application/json" -d "{\"registrationParameters\":{\"fhirNotificationURL\": \"${HOSPITAL_URL}/orca/cpc/fhir/notify\", \"fhirBaseURL\": \"${HOSPITAL_URL}/orca/cpc/fhir\"}}" http://localhost:9081/internal/discovery/v1/dev:HomeMonitoring2024/hospital
# TODO: Remove this init when the Questionnaire is provided by the sub-Task.input
popd

echo "Creating stack for Clinic..."
echo "  Creating devtunnel for Clinic..."
export CLINIC_URL=$(createTunnel ./clinic 7080)
echo "  Clinic url is $CLINIC_URL"
echo "  Starting services"
pushd clinic
echo NUTS_URL="${CLINIC_URL}" > .env
echo CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL="${CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}" >> .env
#docker compose --env-file .env pull
docker compose --env-file .env up --wait --build --remove-orphans
echo "  Creating DID document"
CLINIC_DID=$(createDID "clinic" http://localhost:8081)
echo "    Clinic DID: $CLINIC_DID"
echo "  Self-issuing an NutsUraCredential"
issueUraCredential "clinic" "${CLINIC_DID}"
echo "  Registering on Nuts Discovery Service"
curl -X POST -H "Content-Type: application/json" -d "{\"registrationParameters\":{\"fhirNotificationURL\": \"${CLINIC_URL}/orca/cpc/fhir/notify\", \"fhirBaseURL\": \"${CLINIC_URL}/orca/cpc/fhir\"}}" http://localhost:8081/internal/discovery/v1/dev:HomeMonitoring2024/clinic
echo "  Waiting for the FHIR server to be ready"
popd

# open orchestrator demo app
open "${HOSPITAL_URL}/ehr/"
open "${CLINIC_URL}/viewer/"

# stop Docker services if script exits
trap "docker compose -f clinic/docker-compose.yaml down" EXIT
trap "docker compose -f hospital/docker-compose.yaml down" EXIT
trap "killall devtunnel" EXIT

# tail log of docker compose of both clinic and hospital
docker compose -f clinic/docker-compose.yaml logs -f &
docker compose -f hospital/docker-compose.yaml logs -f



