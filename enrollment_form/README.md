This is a [Next.js](https://nextjs.org/) project bootstrapped with [`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).

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

> ⓘ **Optional Step**: If you do not configure the secrets step below and the terminology server does require auth, the Questionnaire renderer will simply show a "Unable to fetch" message in the form. The application will still function without this step.

Modify the [`./.env.secrets`](./.env.secrets), make sure the `USERNAME_NTS` and `PASSWORD_NTS` point to valid credentials for the Dutch national terminology server. 
See [this](https://nictiz.nl/publicaties/nationale-terminologie-server-handleiding-voor-nieuwe-gebruikers/) manual on how to connect.

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

