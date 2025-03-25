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
## Configuring CarePlanServices
This viewer is can receive resources from multiple CarePlanServices. Therefore, listing CarePlans can be done in a multi-tenant manner. To add a CarePlanService, add the following properties to the configuration:
```
<CPS_NAME>_CAREPLANSERVICE_URL
<CPS_NAME>_BEARER_TOKEN
```