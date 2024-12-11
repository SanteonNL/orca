#!/bin/bash
set -e

# read the tunnel URL from the tunnel log file. It takes the party name (clinic/hospital).
function readTunnelURL() {
  local tunnelLogFile=$1/data/tunnel.log
  TUNNEL_URL=$(grep "Connect via browser:" $tunnelLogFile | awk '{print $4}')
  echo $TUNNEL_URL
}

CLINIC_URL=$(readTunnelURL ./clinic)
pushd clinic
docker compose pull
NUTS_URL="${CLINIC_URL}" \
  docker compose up \
 --wait --build --remove-orphans
 popd

HOSPITAL_URL=$(readTunnelURL ./hospital)
CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL="${HOSPITAL_URL}/orca/cps"
pushd hospital
docker compose pull
NUTS_URL="${HOSPITAL_URL}" \
 CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL="${CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}" \
  docker compose up \
 --wait --build --remove-orphans
popd
