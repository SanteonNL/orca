## Configuration
Use the following environment variables to configure the orchestrator:

- `ORCA_PUBLIC_ADDRESS`: address the public endpoints bind to (default: `:8080`).
- `ORCA_PUBLIC_BASEURL`: base URL of the public endpoints (default: `/`). Set in case the orchestrator is exposed on another path than the domain root.
- `ORCA_NUTS_PUBLIC_URL`: public URL of the Nuts, used for informing OAuth2 clients of the URL of the OAuth2 Authorization Server, e.g. `http://example.com/nuts`. 
- `ORCA_NUTS_API_URL`: address of the Nuts node API to use, e.g. `http://nutsnode:8081`.
- `ORCA_NUTS_SUBJECT`: Nuts subject of the local party, as it was created in/by the Nuts node.
- `ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL`: FHIR base URL of the CarePlan service.
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_URL`: Base URL of the FHIR API the CPC uses for storage.
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_TYPE`: Authentication type for the CPC FHIR store, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity).
- `ORCA_CAREPLANSERVICE_ENABLED`: Enable the CPS (default: `false`).
- `ORCA_CAREPLANSERVICE_FHIR_URL`: Base URL of the FHIR API the CPS uses for storage.
- `ORCA_CAREPLANSERVICE_FHIR_AUTH_TYPE`: Authentication type for the CPS FHIR store, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity). 
- `ORCA_APPLAUNCH_SOF_REDIRECTURI`: SMART App launch redirect URI that is used to send the `code` to by the EHR
- `ORCA_APPLAUNCH_SOF_CLIENTID`:  The `client_id` assigned by the EHR
- `ORCA_APPLAUNCH_SOF_CLIENT_SECRET`: The `client_secret` assigned by the EHR
- `ORCA_APPLAUNCH_SOF_SCOPE`: Any specific scope, for example `launch fhirUser`
- `ORCA_APPLAUNCH_DEMO_ENABLED`: Enable the demo app launch endpoint (default: `false`).
- `ORCA_APPLAUNCH_DEMO_FHIRPROXYURL`: Enable FHIR proxy for demo purposes on `/demo/fhirproxy`, which proxies requests to this URL. 

## App Launch options

### Demo

Redirect the browser to `/demo-app-launch`, and provide the following query parameters:

- `patient`: reference to the FHIR Patient resource.
- `servieRequest`: reference to the FHIR ServiceRequest resource that is being requested.
- `practitioner`: reference to the FHIR PractitionerRole resource of the current user.
- `iss`: FHIR server base URL.

### SMART on FHIR