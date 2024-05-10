# SMART on FHIR Backend Adapter
This application proxies incoming FHIR data exchanges to a SMART on FHIR-enabled XIS as part of an Orca deployment.
It authenticates to the SMART on FHIR API through SMART on FHIR OAuth2 Backend.

It is originally developed to work with the Epic on FHIR API, but should be usable for other SMART on FHIR-enabled XISs as well.

## Usage
- Register the OAuth2 client at the SMART on FHIR-enabled XIS as OAuth2 Backend client.
- Create a .env file with `SOF_BACKEND_ADAPTER_OAUTH_CLIENT_ID` and `SOF_BACKEND_ADAPTER_JWK_KEYID`
- Store the private key which the OAuth2 client uses to sign JWT in `.envfiles/private.jwk`

## Epic
Implementation guide: https://fhir.epic.com/Documentation?docId=oauth2&section=BackendOAuth2Guide
Sandbox URLs: https://open.epic.com/MyApps/Endpoints

Sandbox R4 conformance statement: https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4/metadata
SMART Configuration: https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4/.well-known/smart-configuration

## SMART on FHIR Sandbox App Launch