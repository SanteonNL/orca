#!/bin/bash
set -e

pushd clinic
docker compose --env-file .env pull
docker compose --env-file .env up --wait --build --remove-orphans
popd

pushd hospital
docker compose --env-file .env pull
docker compose --env-file .env up --wait --build --remove-orphans
popd
