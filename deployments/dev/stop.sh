#!/bin/bash

source ~/.bashrc
set -e

pushd clinic
docker compose stop
popd
pushd hospital
docker compose stop
popd
