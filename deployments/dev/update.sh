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
CLINIC_URL="${CLINIC_URL}" \
  docker compose up \
 --wait --build --remove-orphans
 popd

CAREPLANSERVICE_URL="${CLINIC_URL}/fhir"
HOSPITAL_URL=$(readTunnelURL ./hospital)
pushd hospital
NUTS_URL="${HOSPITAL_URL}" \
 CAREPLANSERVICE_URL="${CAREPLANSERVICE_URL}" \
  docker compose up \
 --wait --build --remove-orphans
popd
