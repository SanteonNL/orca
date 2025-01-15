## Configuration
Use the following environment variables to configure the orchestrator:

### General configuration
- `ORCA_PUBLIC_BASEURL` (required): base URL of the public endpoints.
- `ORCA_PUBLIC_ADDRESS` (required): address the public endpoints bind to (default: `:8080`).
- `ORCA_LOGLEVEL`: log level, can be `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, or `disabled` (default: `info`).
- `ORCA_STRICTMODE`: enables strict mode which is recommended in production. (default: `true`).
   Disabling strict mode will change the behavior of the orchestrator in the following ways:
   - Zorgplatform app launch: patient BSN `999911120` is changed to `999999151` (to cope with a bug in its test data).

#### Required configuration for Nuts
- `ORCA_NUTS_PUBLIC_URL`: public URL of the Nuts, used for informing OAuth2 clients of the URL of the OAuth2 Authorization Server, e.g. `http://example.com/nuts`.
- `ORCA_NUTS_API_URL`: address of the Nuts node API to use, e.g. `http://nutsnode:8081`.
- `ORCA_NUTS_SUBJECT`: Nuts subject of the local party, as it was created in/by the Nuts node.
- `ORCA_NUTS_DISCOVERYSERVICE`: ID of the Nuts Discovery Service that is used for CSD lookups (finding (local) care organizations and looking up their endpoints).
- `ORCA_NUTS_AZUREKV_URL`: URL of the Azure Key Vault that holds the client certificate for outbound HTTP requests.
- `ORCA_NUTS_AZUREKV_CLIENTCERTNAME`: Name of the certificate for outbound HTTP requests.

### Care Plan Contributor configuration
- `ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL`: FHIR base URL of the CarePlan service.
- `ORCA_CAREPLANCONTRIBUTOR_STATICBEARERTOKEN`: Secures the EHR-facing endpoints with a static HTTP Bearer token. Only intended for development and testing purposes, since they're unpractical to change often.
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_URL`: Base URL of the FHIR API the CPC uses for storage.
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_TYPE`: Authentication type for the CPC FHIR store, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity).
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_SCOPES`: OAuth2 scopes to request when authenticating with the FHIR server. If no scopes are provided, the default scope might be used, depending on the authentication method (e.g. Azure default scope).
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_REDIRECTURI`: SMART App launch redirect URI that is used to send the `code` to by the EHR
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_CLIENTID`:  The `client_id` assigned by the EHR
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_CLIENT_SECRET`: The `client_secret` assigned by the EHR
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_SCOPE`: Any specific scope, for example `launch fhirUser`
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_DEMO_ENABLED`: Enable the demo app launch endpoint (default: `false`).
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_DEMO_FHIRPROXYURL`: Enable FHIR proxy for demo purposes on `/demo/fhirproxy`, which proxies requests to this URL.
- `ORCA_CAREPLANCONTRIBUTOR_FRONTEND_URL`: Base URL of the frontend application, to which the browser is redirected on app launch (default: `/frontend/enrollment`).
- `ORCA_CAREPLANCONTRIBUTOR_SESSIONTIMEOUT`: Configure the user session timeout, use Golang time.Duration format (default: 15m).

### Care Plan Contributor Task Filler configuration
The Task Filler engine determines what Tasks to accept and what information is needed to fulfill them through FHIR HealthcareService and Questionnaire resources.
These FHIR resources can be read from a different FHIR API than configured in `ORCA_CAREPLANCONTRIBUTOR_QUESTIONNAIREFHIR` by setting 
`ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_URL`, `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_AUTH_TYPE` and `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_AUTH_SCOPES`.

If you want to automatically load FHIR HealthcareService and Questionnaire resources into the FHIR API on startup,
you can configure the Task Filler to do so by setting `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS`.
It takes a list (separated by commas) of URLs to fetch the FHIR Bundles from, that will be loaded into the FHIR API.
The bundles may only contain Questionnaire and HealthcareService resources.

### Care Plan Service configuration
- `ORCA_CAREPLANSERVICE_ENABLED`: Enable the CPS (default: `false`).
- `ORCA_CAREPLANSERVICE_FHIR_URL`: Base URL of the FHIR API the CPS uses for storage.
- `ORCA_CAREPLANSERVICE_FHIR_AUTH_TYPE`: Authentication type for the CPS FHIR store, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity).
- `ORCA_CAREPLANSERVICE_FHIR_AUTH_SCOPES`: OAuth2 scopes to request when authenticating with the FHIR server. If no scopes are provided, the default scope might be used, depending on the authentication method (e.g. Azure default scope).

### Kafka / Eventhubs configuration

* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_ENABLED`: Determines whether Kafka integration is enabled for the CarePlan Contributor component of ORCA. If set to `false`, all other Kafka configuration options will be ignored.
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_DEBUG`: Determines whether Kafka debug option is enabled for the CarePlan Contributor component of ORCA. This will log all Kafka messages to the /tmp directory. Note that this setting should only be enabled for debugging purposes and once set, all other configuration options below will be ignored. The value of `ORCA_CAREPLANCONTRIBUTOR_KAFKA_ENABLED` needs to be set to `true` to enable this feature.
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_TOPIC`: Specifies the Kafka topic to which patient enrollment events are published.
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_ENDPOINT`: Defines the Kafka broker URL to connect to for publishing and consuming messages.
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_SASL_MECHANISM`: Specifies the SASL mechanism used for Kafka authentication. The current implementation only supports `PLAIN`.
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_SECURITY_PROTOCOL`: Specifies the security protocol used for Kafka communication. The current implementation only supports `SASL_PLAINTEXT`.
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_SASL_USERNAME`: The username or connection string used for authenticating with the Kafka broker. 
* `ORCA_CAREPLANCONTRIBUTOR_KAFKA_SASL_PASSWORD`: The password or secret key used in conjunction with the `ORCA_CAREPLANCONTRIBUTOR_KAFKA_SASL_USERNAME` for Kafka authentication.

## App Launch options

### Demo

Redirect the browser to `/demo-app-launch`, and provide the following query parameters:

- `patient`: reference to the FHIR Patient resource.
- `servieRequest`: reference to the FHIR ServiceRequest resource that is being requested.
- `practitioner`: reference to the FHIR PractitionerRole resource of the current user.
- `iss`: FHIR server base URL.

### SMART on FHIR