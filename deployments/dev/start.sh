#!/bin/bash

set -e

docker compose pull
docker compose up --wait --build --remove-orphans

source ./init.sh

# open orchestrator demo app
open "http://localhost:8081/ehr/"
open "http://localhost:8080/viewer/"

# stop Docker services if script exits
trap "docker compose docker-compose.yaml down" EXIT

# show logs
docker compose -f docker-compose.yaml logs -f &



