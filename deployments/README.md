# Orca Deployments
This directory contains example Orca deployments.

## dev
A development environment for testing, developing or demo-ing Orca services. It uses FHIR data from the public SMART on FHIR Sandbox.
Is uses Microsoft DevTunnel to make DIDs resolvable.
Pre-requisites:
- Docker Compose
- Bash
- Microsoft DevTunnel (e.g. log in with GitHub using `devtunnel user login -d -g`)
- jq (`brew install jq`)

Use `start.sh` inside the directory to start the stack. It will open the orchestrator demo page when it has been started.

### Testing Zorgplatform
In order to test Zorgplatform, follow these steps

1. Add the keys that are mounted in the Hospital [docker-compose.yaml](./dev/hospital/docker-compose.yaml) 
2. Uncomment the `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_*` values and fill in the <...> values