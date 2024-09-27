.. _deployment-docker-compose:

Running with Docker Compose
###########################

Thuis guide helps you to run the Orca node in Docker with Docker Compose.

Examples
********

Example Hospital

.. code-block:: yaml

    services:
        proxy:
            image: nginx:latest
            depends_on:
            - nutsnode
            - orchestrator
            - fhirstore
            ports:
            - "7080:8080"
            volumes:
            - "./config/nginx-proxy.conf:/etc/nginx/conf.d/nginx-proxy.conf:ro"
        nutsnode:
            image: nutsfoundation/nuts-node:6.0.0-beta.13
            ports:
            - "8081:8081" # port exposed for loading test data (create DIDs and VCs, requesting access token)
            volumes:
            - "../shared_config/policy/:/nuts/config/policy:ro"
            - "../shared_config/discovery/homemonitoring.json:/nuts/discovery/homemonitoring.json:ro"
            environment:
            - NUTS_VERBOSITY=debug
            - NUTS_STRICTMODE=false
            - NUTS_HTTP_INTERNAL_ADDRESS=:8081
            - NUTS_AUTH_CONTRACTVALIDATORS=dummy
            - NUTS_URL=${NUTS_URL}
            - NUTS_DISCOVERY_DEFINITIONS_DIRECTORY=/nuts/discovery/
            - NUTS_DISCOVERY_SERVER_IDS=dev:HomeMonitoring2024
            - NUTS_VDR_DIDMETHODS=web
        nutsadmin:
            image: "nutsfoundation/nuts-admin:main"
            environment:
            - NUTS_NODE_ADDRESS=http://nutsnode:8081
            ports:
            - "1305:1305"
        orchestrator:
            image: ghcr.io/santeonnl/orca_orchestrator:main
            # build:
            #  context: ../../../orchestrator
            #  dockerfile: Dockerfile
            ports:
            - "8080"
            environment:
            - ORCA_NUTS_API_URL=http://nutsnode:8081
            - ORCA_NUTS_PUBLIC_URL=${NUTS_URL}
            - ORCA_NUTS_SUBJECT=clinic
            - ORCA_PUBLIC_URL=/orca
            - ORCA_CAREPLANSERVICE_ENABLED=true
            - ORCA_CAREPLANSERVICE_FHIR_URL=http://fhirstore:8080/fhir
            - ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL=${CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}
        fhirstore:
            image: hapiproject/hapi:v7.2.0
            ports:
            - "7090:8080"
            environment:
            - hapi.fhir.fhir_version=R4
        viewer:
            image: ghcr.io/santeonnl/orca_viewersimulator:main
            # build:
            #   context: ../../../viewer_simulator
            #   dockerfile: Dockerfile
