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
- `ORCA_NUTS_AZUREKV_CREDENTIALTYPE`: Type of the credential for the Azure Key Vault, options: `managed_identity`, `cli`, `default` (default: `managed_identity`).

### Care Plan Contributor configuration
- `ORCA_CAREPLANCONTRIBUTOR_STATICBEARERTOKEN`: Secures the EHR-facing endpoints with a static HTTP Bearer token. Only intended for development and testing purposes, since they're unpractical to change often.
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_URL`: Base URL of the FHIR API the CPC uses for storage. When `ORCA_CAREPLANCONTRIBUTOR_HEALTHDATAVIEWENDPOINTENABLED` is enabled, data is retrieved from this FHIR API.
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_TYPE`: Authentication type for the CPC FHIR store, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity).
- `ORCA_CAREPLANCONTRIBUTOR_FHIR_AUTH_SCOPES`: OAuth2 scopes to request when authenticating with the FHIR server. If no scopes are provided, the default scope might be used, depending on the authentication method (e.g. Azure default scope).
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_DEMO_ENABLED`: Enable the demo app launch endpoint (default: `false`).
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_DEMO_FHIRPROXYURL`: Enable FHIR proxy for demo purposes on `/demo/fhirproxy`, which proxies requests to this URL.
- `ORCA_CAREPLANCONTRIBUTOR_FRONTEND_URL`: Base URL of the frontend application, to which the browser is redirected on app launch (default: `/frontend/enrollment`).
- `ORCA_CAREPLANCONTRIBUTOR_SESSIONTIMEOUT`: Configure the user session timeout, use Golang time.Duration format (default: 15m).

#### OpenID Connect Provider Configuration
ORCA can act as OpenID Connect Provider for users that have an existing session (initiated through app launch).
This allows the launch of OIDC-enabled applications that can't directly authenticate using the EHR.
It supports the following scopes:
- `openid`: required, adds the `sub`  and `orgid` claims.
  - The `sub` claim contains the user identifier unique to the integrated EHR. Its format depends on the EHR (e.g., for ChipSoft HiX it's `<user id>@<HL7 NL OID>`).
  - The `orgid` claim contains an array of organization identifiers (string) for which the user is authenticated in HL7 FHIR token format.
    It follows the following format: `<system>|<value>`, where the `system` depends on the SCP profile.
    Note: currently, the system of the identifier will always be URA (`http://fhir.nl/fhir/NamingSystem/ura`), since it's the only supported profile.
- `profile`: adds the `name` and `roles` claims.
- `email`: adds the `email` claim.
- `patient`: adds `patient` claim, which contains an array with identifiers of the patient associated with the ORCA user session. The format of the identifiers is `<system>|<value>`.

The claims in the ID token are based on the user information from the EHR.

To configure the OIDC Provider, set the following environment variables:
- `ORCA_CAREPLANCONTRIBUTOR_OIDC_ENABLED`: Enables the OIDC provider (default: `false`).

To register a client (application), set the following environment variables for that client:
- `ORCA_CAREPLANCONTRIBUTOR_OIDC_CLIENTS_<clientname>_ID`: ID of the client, which will be used as `client_id` in the OIDC flow.
- `ORCA_CAREPLANCONTRIBUTOR_OIDC_CLIENTS_<clientname>_REDIRECTURI`: URL to which the OIDC provider will redirect the user after authentication.

#### Care Plan Contributor Task Filler configuration
The Task Filler engine determines what Tasks to accept and what information is needed to fulfill them through FHIR HealthcareService and Questionnaire resources.
You have the following options:

1. Read them from your FHIR API
2. Synchronize configured URLs with your FHIR API, then query your FHIR API
3. Read them from configured URLs and only keep them in-memory.

The most robust options (1 and 2) query the resources from your FHIR API. You can automate updating them by synchronizing them on startup.
Configure these options to achieve this:
- FHIR API for Questionnaire and HealthcareService resources:
  - `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_URL`: Base URL of the FHIR API for querying Questionnaire and HealthcareService resources.
  - `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_AUTH_TYPE`: Authentication type for the FHIR API, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity).
  - `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_AUTH_SCOPES`: OAuth2 scopes to request when authenticating with the FHIR server. If no scopes are provided, the default scope might be used, depending on the authentication method (e.g. Azure default scope).
- `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS`: Only if you want to synchronize on startup: a list of comma-separated URLs to fetch the FHIR Bundles from, that will be loaded into the FHIR API.
  It will only load FHIR Questionnaire and HealthcareService resources.

If you don't want to query the FHIR Questionnaire and HealthcareService resources from your FHIR API, only set `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS`.
The downside of this option is that the resources MUST be available on startup.

##### EHR integration
If you want to receive accepted tasks in your EHR, you can set `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_TASKACCEPTEDBUNDLETOPIC`
to the messaging topic or queue on which the task bundle will be delivered. You will also need to create the `orca.taskengine.task-accepted` on your broker.

See "Messaging configuration" for more information.

#### External application discovery
If you have web applications that you want other care organizations to discovery through ORCA, you can set the following options:
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_EXTERNAL_<KEY>_NAME`: Name of the external application.
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_EXTERNAL_<KEY>_URL`: URL of the external application.

These configured applications are discovered by searching for FHIR Endpoints on the CPC's FHIR Endpoint.
Note: this endpoint only supports searching using HTTP GET, without query parameters.

### Care Plan Service configuration
- `ORCA_CAREPLANSERVICE_ENABLED`: Enable the CPS (default: `false`).
- `ORCA_CAREPLANSERVICE_FHIR_URL`: Base URL of the FHIR API the CPS uses for storage.
- `ORCA_CAREPLANSERVICE_FHIR_AUTH_TYPE`: Authentication type for the CPS FHIR store, options: `` (empty, no authentication), `azure-managedidentity` (Azure Managed Identity).
- `ORCA_CAREPLANSERVICE_FHIR_AUTH_SCOPES`: OAuth2 scopes to request when authenticating with the FHIR server. If no scopes are provided, the default scope might be used, depending on the authentication method (e.g. Azure default scope).
- `ORCA_CAREPLANSERVICE_EVENTS_WEBHOOK_URL`: URL to which the CPS sends webhooks when a CarePlan is created. It sends the CarePlan resource as HTTP POST request with content type `application/json`.

### Messaging configuration
Application event handling and FHIR Subscription notification sending uses a message broker.
By default, an in-memory message broker is used, which doesn't retry messages.
For production environments, it's recommended to use Azure ServiceBus.

* `ORCA_MESSAGING_AZURESERVICEBUS_HOSTNAME`: The hostname of the Azure ServiceBus instance, setting this (or the connection string) enables use of Azure ServiceBus as message broker.
* `ORCA_MESSAGING_AZURESERVICEBUS_CONNECTIONSTRING`: The connection string of the Azure ServiceBus instance, setting this (or the hostname) enables use of Azure ServiceBus as message broker.
* `ORCA_MESSAGING_ENTITYPREFIX`: Optional prefix for topics and queues, which allows multi-tenancy (using the same underlying message broker infrastructure for multiple ORCA instances) by prefixing the entity names with a tenant identifier.
* `ORCA_MESSAGING_HTTP_ENDPOINT`: For demo purposes: a URL pointing HTTP endpoint, to which messages will also be delivered. It appends the topic name to this URL.
* `ORCA_MESSAGING_HTTP_TOPICFILTER`: For demo purposes: topics to enable the HTTP endpoint for (separator: `,`). If not set, all topics are enabled.

If you're Azure Service Bus, depending on the features you've enabled, you'll need to create the following queues: 

- Queue `orca.taskengine.task-accepted` (if `ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_TASKACCEPTEDBUNDLETOPIC` is set).
- Queue `orca.hl7.fhir.careplan-created` (if `ORCA_CAREPLANSERVICE_EVENTS_WEBHOOK_URL` is set).
- Queue `orca.subscriptionmgr.notification` (if `ORCA_CAREPLANSERVICE_ENABLED` is `true`).

## App Launch options

### Demo

Redirect the browser to `/demo-app-launch`, and provide the following query parameters:

- `patient`: reference to the FHIR Patient resource.
- `servieRequest`: reference to the FHIR ServiceRequest resource that is being requested.
- `practitioner`: reference to the FHIR PractitionerRole resource of the current user.
- `iss`: FHIR server base URL.

### SMART on FHIR

Currently not up-to-date, it probably won't work.

- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_REDIRECTURI`: SMART App launch redirect URI that is used to send the `code` to by the EHR
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_CLIENTID`:  The `client_id` assigned by the EHR
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_CLIENT_SECRET`: The `client_secret` assigned by the EHR
- `ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_SOF_SCOPE`: Any specific scope, for example `launch fhirUser`