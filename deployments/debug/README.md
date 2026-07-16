# Running Orca locally (debug stack)

This directory contains a **debug** deployment: the supporting infrastructure
(FHIR store, Nuts node, Aspire dashboard, reverse proxies) runs in Docker, while the
Orca applications (orchestrators, frontend, hospital EHR simulator) run directly on your
machine via the VS Code debugger. That split lets you set breakpoints and step through
the Go/Next.js code — ideal for debugging orchestration logic and testing OpenTelemetry
tracing locally.

> Want everything in containers with no debugger? Use [`../dev`](../dev) instead
> (`docker compose up`). Use **this** `debug` stack when you need breakpoints or want to
> inspect traces in the Aspire dashboard.

## What runs where

| Component | Runs in | Address |
|---|---|---|
| Aspire dashboard (traces UI) | Docker | http://localhost:18888 |
| Aspire OTLP ingest (gRPC) | Docker | `localhost:18889` |
| HAPI FHIR store | Docker | http://localhost:9090/fhir |
| Nuts node (internal API) | Docker | http://localhost:9081 |
| Nuts admin UI | Docker | http://localhost:1405 |
| Hospital reverse proxy | Docker | http://localhost:8081 |
| Clinic reverse proxy | Docker | http://localhost:8082 |
| Hospital orchestrator | VS Code | http://localhost:8090 |
| Clinic orchestrator | VS Code | http://localhost:8091 |
| Hospital frontend (enrollment UI) | VS Code | http://localhost:3003 |
| Hospital EHR simulator | VS Code | http://localhost:3001 |

Primary entry point once everything is running: **Hospital EHR → http://localhost:8081/ehr**

## Prerequisites

- Docker Desktop (running)
- Go (see `orchestrator/go.mod` for the version)
- Node.js + npm/pnpm (for the frontend and hospital simulator)
- VS Code with the Go and JavaScript debugger extensions

## Setup

### 1. Create the `.env` file (one-time)

The debug stack's `docker-compose.yaml` references `${orca_dev_base}` to locate its mounted
config (Nuts policy, discovery, nginx config). Unlike the `dev` stack, `deployments/debug/.env`
is **git-ignored**, so you must create it yourself:

```bash
# from deployments/debug/
echo "orca_dev_base=." > .env
```

Without this, `docker compose` warns `The "orca_dev_base" variable is not set` and the
Nuts node / proxies fail to mount their config.

### 2. Enable the VS Code launch configs (one-time)

If your checkout only has the `.default` templates, copy them:

```bash
# from the repo root
cp .vscode/launch.default.json .vscode/launch.json
cp .vscode/tasks.default.json  .vscode/tasks.json
```

(If `.vscode/launch.json` and `.vscode/tasks.json` already exist, skip this.)

### 3. Start the Docker infrastructure

```bash
# from deployments/debug/
docker compose up -d
```

Wait for the stack to settle, then sanity-check the key endpoints:

```bash
curl -o /dev/null -w "fhirstore: %{http_code}\n" http://localhost:9090/fhir/metadata   # expect 200
curl -o /dev/null -w "nutsnode:  %{http_code}\n" http://localhost:9081/status           # expect 200
curl -o /dev/null -w "aspire:    %{http_code}\n" http://localhost:18888/                # expect 302
```

Confirm the Nuts bootstrap succeeded — the `nutsnode-init` container should exit `0` after
creating the Hospital and Clinic DIDs:

```bash
docker compose logs nutsnode-init | tail
# expect: "Creating stack for Hospital..." / "Creating stack for Clinic..." and exit code 0
```

> The `.vscode` "Launch Hospital & Clinic Applications" compound has a `Prepare Debug`
> pre-launch task that also runs `docker compose up -d` for you. Running it manually first
> (as above) makes failures easier to spot.

### 4. Start the applications in VS Code

Open **Run and Debug** → run the **"Launch Hospital & Clinic Applications"** compound
configuration. This starts the hospital frontend, hospital EHR simulator, both
orchestrators, and a Chrome debug session — all wired to the Dockerized infrastructure and
exporting traces to the Aspire dashboard.

Open **http://localhost:8081/ehr** to drive the flow.

## Observability / OpenTelemetry

The orchestrators export OTLP traces over gRPC to the Aspire dashboard
(`OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:18889`). Open **http://localhost:18888** →
**Traces** to inspect spans, their attributes, and their status.

See [Why we use `span.SetAttributes` instead of `span.AddEvent`](#why-we-use-spansetattributes-instead-of-spanaddevent)
below before adding new trace instrumentation.

## Troubleshooting

### `nutsnode-init` exits with code 2 / `illegal option -` / DIDs not created

Symptom in `docker compose logs nutsnode-init`:

```
/nutsnode-init.sh: line 2: : not found
/nutsnode-init.sh: set: line 3: illegal option -
```

Cause: `nuts/nutsnode-init.sh` has Windows **CRLF** line endings, which break the shebang
and `set -e` inside the Linux container. Fix by converting it to **LF** (the repo's
`.gitattributes` enforces `eol=lf` for this file, so a fresh checkout should already be
correct; older checkouts may still have CRLF):

```bash
# from the repo root, restore the committed LF version
git show HEAD:deployments/debug/nuts/nutsnode-init.sh > deployments/debug/nuts/nutsnode-init.sh
docker compose -f deployments/debug/docker-compose.yaml up -d nutsnode-init
```

### `FHIR_BASE_URL is not found` / FHIR calls fail

The FHIR store listens on port **8080 inside the Docker network** and is published to the
host on **9090**. Use the right one for where the code runs:

- **Inside Docker** (the `dev` stack): `http://fhirstore:8080/fhir`
- **On the host** (the `debug` stack, apps in VS Code): `http://localhost:9090/fhir`

`http://fhirstore:9090` and `http://localhost:8080` are both wrong. The VS Code launch
config already sets `FHIR_BASE_URL=http://localhost:9090/fhir` for the host-run apps.

### `The "orca_dev_base" variable is not set`

You skipped step 1 — create `deployments/debug/.env` with `orca_dev_base=.`.

---

## Why we use `span.SetAttributes` instead of `span.AddEvent`

**Short version:** in our Azure Container Apps environment, OpenTelemetry **span events**
(`span.AddEvent(...)`) never reach Application Insights, while **span attributes**
(`span.SetAttributes(...)`) do. So we record trace milestones as attributes.

### The investigation

While adding orchestration logging we found that `span.AddEvent(...)` markers were
completely absent from Application Insights (searching the `traces` table by the event
strings returned nothing), even though `span.SetAttributes(...)` values on the *same spans*
(e.g. `authN.method`, `authN.outcome`) showed up fine.

Testing ruled out our code as the cause:

1. **Our OTEL setup is correct.** A standalone smoke test using the exact same SDK
   (`otlptracegrpc` → `localhost:18889`) sent a span with three `AddEvent` calls; all three
   appeared under the span's **Events** section in the local Aspire dashboard. The
   SDK → OTLP → collector path preserves events perfectly.

2. **The loss happens in Azure, not in the app.** Azure Container Apps' *managed*
   OpenTelemetry agent forwards traces to Application Insights using the community
   [`azuremonitorexporter`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/azuremonitorexporter/README.md).
   That exporter has a setting, **`spaneventsenabled`, which defaults to `false`** — when
   off, span events are dropped and never written to the `traces` table.

3. **We can't turn it on.** Azure Container Apps exposes only a minimal telemetry config
   (a connection string and which signals — logs/metrics/traces — to forward, via
   ARM/Bicep/CLI/Terraform). There is **no knob to set `spaneventsenabled`** on the managed
   agent. See
   [Collect and read OpenTelemetry data in Azure Container Apps](https://learn.microsoft.com/en-us/azure/container-apps/opentelemetry-agents).

### Consequences

- **Span attributes are the reliable channel.** They travel as part of the span itself
  (mapped to `requests`/`dependencies` telemetry with custom dimensions) and always arrive.
- Milestone markers that were `span.AddEvent("something_happened")` are recorded as
  `span.SetAttributes(attribute.Bool(otel.SomethingHappened, true))` instead. Event-name
  constants live in [`orchestrator/lib/otel/events.go`](../../orchestrator/lib/otel/events.go).
- Locally in the **Aspire dashboard**, both events and attributes are visible — so if you
  add `AddEvent` calls you will see them here but they will silently vanish in Application
  Insights. Prefer `SetAttributes`.

### If we ever need span events in Application Insights

Stop routing traces through the managed agent and run a **self-hosted OpenTelemetry
Collector** (e.g. a sidecar) with `azuremonitorexporter` configured `spaneventsenabled: true`,
then point `OTEL_EXPORTER_OTLP_ENDPOINT` at that collector. This is an infrastructure
change, not an application change.
