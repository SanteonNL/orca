#!/bin/bash

echo "Setup environment"
docker compose up proxy nutsnode fhirstore --wait
go run .
if [ $? -ne 0 ]; then
  echo "Set-up failed"
  docker compose stop
  exit 1
fi

echo "Run tests"
docker compose restart nutsnode
docker compose up --wait
go test -v ./... -count=1

if [ $? -ne 0 ]; then
  echo "Tests failed"
  docker compose logs
  docker compose stop
  exit 1
fi

echo "Tests passed"
docker compose stop
exit 0
