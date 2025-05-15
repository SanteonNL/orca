# Hospital Simulator

## Getting Started

### 1. Install dependencies

```
npm install
```

### 2. Configure your local environment

Copy the .env.local.example file in this directory to .env.local (which will be ignored by Git):

```
cp .env.local.example .env.local
```

Add details to the `.env.local` if they need to change. The defaults in the example are set to the local orchestrator and the nuts-node from the [`./docker-compose.yaml`](./docker-compose.yaml).

### 3. Start the application

To run your site locally, use:

```
npm run dev
```

To run it in production mode, use:

```
npm run build
npm run start
```
## Configuration
The viewer is multi-tenant: it can show data from multiple ORCA nodes. The configuration properties for an ORCA are prefixed with the tenant's name.
The following properties are required for each ORCA:
- `<NAME>_ORCA_URL`: The base URL of the ORCA node (without `/cpc` or `/cps` postfix)
- `<NAME>_ORCA_BEARERTOKEN`: The bearer token to authenticate with the ORCA node.
- `<NAME>_IDENTIFIER`: The identifier care organization running the ORCA node, as FHIR token, e.g.: `http://fhir.nl/fhir/NamingSystem/ura|1234`.

The "BgZ View" uses a CarePlan listing to select the patient to show the BgZ data for. This is because this Viewer application doesn't have local storage for keeping track of CarePlans.
You can configure multiple CarePlanServices to fetch CarePlans for, using the `<NAME>_CPS_URL` property.
This property is a comma-separated list of CarePlanService URLs. The CarePlanService URL should be the FHIR base URL of the CarePlanService (e.g. `https://example.com/cps`).