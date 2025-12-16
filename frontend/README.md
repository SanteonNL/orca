# Frontend project for ORCA
This project serves as the front-end of the orchestrator. Next.js has a back-end, but we mainly use the Golang [orchestrator](../orchestrator) project for that. 

This project visualizes steps like:
1. Selecting a `CarePlan` when the enrollment flow is initialized
2. Providing more information from the placer to the filler by rendering Questionnaires. 
3. Pre-populating Questionnaires as much a possible as per [SDC specification](https://hl7.org/fhir/uv/sdc/index.html)

This is a [Next.js](https://nextjs.org/) project bootstrapped with [`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).

## Configuration
The following configuration options are supported:

- `NEXT_PUBLIC_TITLE`: the title of the application, defaults to `ORCA Frontend`.
- `ORCA_PATIENT_IDENTIFIER_SYSTEM`: the FHIR coding system for patient identifiers, defaults to `http://fhir.nl/fhir/NamingSystem/bsn`.
- `SUPPORT_CONTACT_EMAIL`: an email for support resource. It will be shown on error pages.

## Getting Started
### 1. Install dependencies

```
pnpm install
```

### 2. Configure your local environment

Copy the .env.local.example file in this directory to .env.local (which will be ignored by Git):

```
cp .env.local.example .env.local
cp .env.secrets.example .env.secrets
```

Add details to the `.env.local` if they need to change. The defaults in the example are set to the local orchestrator and the nuts-node from the [dev deployment docker-compose](/deployments/dev/hospital/docker-compose.yaml).

### 3. Start the application
#### Dev mode
To run your site locally, use:

```
pnpm run dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

#### Production mode

> :warning: **Use the dev deployment**: You *can* build this locally, but ideally you run the project from the [dev deployment](/deployments/dev/start.sh)

Otherwise, use:
```
pnpm run build
pnpm run start
```


## Running locally
http://localhost:9090/fhir/Patient
http://localhost:9090/fhir/ServiceRequest

http://localhost:3000/enrollment/new