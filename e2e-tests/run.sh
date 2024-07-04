#!/bin/bash

go test -v ./...

if [ $? -ne 0 ]; then
  echo "Tests failed"
  docker compose logs
  docker compose down
  exit 1
fi

echo "Tests passed"
docker compose down
exit 0
