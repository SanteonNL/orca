# SMART on FHIR Backend Adapter
This application proxies incoming FHIR data exchanges to a SMART on FHIR-enabled XIS as part of an Orca deployment.
It authenticates to the SMART on FHIR API through SMART on FHIR OAuth2 Backend.

It can use a private key in JWK format from a local file to sign JWTs for OAuth2 authentication,
or use a JWK from Azure Key Vault.

It is originally developed to work with the Epic on FHIR API, but should be usable for other SMART on FHIR-enabled XISs as well.

## Supported Key Types
Only EC keys are supported.

## Usage
Make sure the application is registered as OAuth2 client at the SMART on FHIR-enabled XIS as OAuth2 Backend client.

Provide the following environment variables (and add the ones required depending on the JWK source):
- `SOF_BACKEND_ADAPTER_LISTEN_ADDRESS`: address to listen on, e.g. `:8080`. Make sure to expose/map this port in the Docker container.
- `SOF_BACKEND_ADAPTER_FHIR_BASEURL`: base URL of the FHIR API of the SMART on FHIR-enabled XIS to proxy to.
- `SOF_BACKEND_ADAPTER_OAUTH_CLIENT_ID`: OAuth2 client ID of the application as registered.

### Usage with local JWK
Provide `SOF_BACKEND_ADAPTER_SIGNINGKEY_FILE` (see `../deployments/dev/hospital/docker-compose.yaml`).
Note that the JWK must contain a key id (`kid`) and signing algorithm (`alg`).

### Usage with Azure Key Vault
Provide the following environment variables for resolving the JWK from Azure Key Vault:
- `SOF_BACKEND_ADAPTER_AZURE_KEYVAULT_URL`: URL of the Azure Key Vault.
- `SOF_BACKEND_ADAPTER_SIGNINGKEY_AZURE_KEYNAME`: name of the key in the Azure Key Vault.

The application will use the default credential with secret to authenticate to Azure Key Vault,
so provide the following environment variables for authentication to Azure Key Vault:
- `AZURE_TENANT_ID`: tenant ID of the Azure Key Vault.
- `AZURE_CLIENT_ID`: client ID of the Azure Key Vault.
- `AZURE_CLIENT_SECRET`: client secret of the Azure Key Vault.

## Testing with the Epic on FHIR Sandbox
Implementation guide: https://fhir.epic.com/Documentation?docId=oauth2&section=BackendOAuth2Guide
Sandbox URLs: https://open.epic.com/MyApps/Endpoints

Sandbox R4 conformance statement: https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4/metadata
SMART Configuration: https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4/.well-known/smart-configuration