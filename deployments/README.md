# Orca Deployments
This directory contains example Orca deployments.

## dev
Run `docker compose up` to start the development environment in the `dev` directory.

### Testing Zorgplatform
In order to test Zorgplatform, follow these steps

1. Add the keys that are mounted in the Hospital [docker-compose.yaml](./dev/hospital/docker-compose.yaml) 
2. Uncomment the `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_*` values and fill in the <...> values