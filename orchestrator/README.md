## Configuration
Use the following environment variables to configure the orchestrator:

- `ORCA_PUBLIC_ADDRESS`: address the public endpoints bind to (default: `:8080`).
- `ORCA_NUTS_API_ADDRESS`: address of the Nuts node API to use, e.g. `http://nutsnode:8081`.
- `ORCA_CAREPLANSERVICE_URL`: FHIR base URL of the CarePlan service.
- `ORCA_URAMAP`: hardcoded map of URA to DID, e.g. `1234=did:web:example.com:1234,5678=did:web:example.com:5678`.
- `ORCA_APPLAUNCH_SOF_REDIRECTURI`: SMART App launch redirect URI that is used to send the `code` to by the EHR
- `ORCA_APPLAUNCH_SOF_CLIENTID`:  The `client_id` assigned by the EHR
- `ORCA_APPLAUNCH_SOF_CLIENT_SECRET`: The `client_secret` assigned by the EHR
- `ORCA_APPLAUNCH_SOF_SCOPE`: Any specific scope, for example `launch fhirUser`

## App Launch options

### Demo

Redirect the browser to `/demo-app-launch`, and provide the following query parameters:

- `patient`: reference to the FHIR Patient resource.
- `servieRequest`: reference to the FHIR ServiceRequest resource that is being requested.
- `practitioner`: reference to the FHIR PractitionerRole resource of the current user.
- `iss`: FHIR server base URL.

### SMART on FHIR