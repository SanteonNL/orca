# Running Orca locally (dev stack)

This directory runs **everything in containers** — the Nuts node, FHIR store, both
orchestrators, the enrollment frontend, the hospital EHR simulator and the clinic viewer.
Use it to exercise a flow end-to-end without a debugger attached.

> Need breakpoints or trace inspection? Use [`../debug`](../debug) instead, which runs the
> Go/Next.js applications from VS Code against Dockerized infrastructure.

## Prerequisites

- Docker Desktop (running)

## Start

```bash
# from deployments/dev/ — the .env sets orca_dev_base=., so the relative
# config mounts only resolve when compose runs from this directory
docker compose up -d --build
```

The first build takes several minutes (two Go binaries and three Next.js apps).

## What runs where

| Component | Address |
|---|---|
| **Hospital EHR simulator** (start here) | http://localhost:8081/ehr |
| Clinic viewer (task receiver) | http://localhost:8082/viewer |
| Enrollment frontend (via hospital proxy) | http://localhost:8081/frontend |
| Hospital orchestrator | http://localhost:8090 (API only, no page at `/`) |
| HAPI FHIR store | http://localhost:9090/fhir |
| Nuts node (internal API) | http://localhost:9081 |
| Nuts admin UI | http://localhost:1405 |
| Aspire dashboard (traces) | http://localhost:18888 |

## Sanity checks

```bash
curl -o /dev/null -w "fhirstore: %{http_code}\n" http://localhost:9090/fhir/metadata  # 200
curl -o /dev/null -w "nutsnode:  %{http_code}\n" http://localhost:9081/status          # 200
curl -o /dev/null -w "ehr:       %{http_code}\n" http://localhost:8081/ehr             # 200

# Both organizations must have a DID, or every app launch fails
curl -s http://localhost:9081/internal/vdr/v2/subject
# expect: {"clinic":["did:web:..."],"hospital":["did:web:..."]}
```

## Walkthrough: enrolling a patient

1. Open the hospital EHR simulator at http://localhost:8081/ehr.
2. **+** → fill in the patient (the phone and email fields are pre-filled with valid
   defaults; overwrite them to test contact-detail validation) → **Create**.
3. Click the clipboard icon in the patient's **Service Requests** column.
4. **+** → pick a service and condition → **Create**.
5. Click the **Enroll** icon on the ServiceRequest row.

Step 5 opens the enrollment frontend in a **popup window**, so allow popups for
`localhost:8081`. If the popup is blocked, or you are driving the flow from a script,
build the launch URL yourself:

```bash
# ids come from the FHIR store: /Patient, /Practitioner, /ServiceRequest
open "http://localhost:8081/orca/demo-app-launch?tenant=hospital\
&patient=Patient%2F<id>&practitioner=Practitioner%2F<id>&serviceRequest=ServiceRequest%2F<id>"
```

The first screen (*Controleer patiëntgegevens*) shows the patient's contact details.
Pressing **Volgende stap** creates the Patient at the CarePlanService — this is where
contact-detail validation runs.

## Testing patient contact-detail validation

`orchestrator/careplanservice/validate_patient.go` gates enrollment on the patient's
email and phone number, and reports failures as codes that
`frontend/lib/fhirUtils.ts` turns into the Dutch banner on the confirmation screen:

| Code | Meaning |
|---|---|
| `E0001` / `E0003` | email missing / not parseable |
| `E0002` | no phone number at all |
| `E0004` | a phone number is present, but none of them is usable |

A number is usable when it normalizes to valid E.164 (`+<country><subscriber>`); a leading
`00` is rewritten to `+` and a Dutch `06…` to `+316…`. Dutch numbers additionally have to
be mobile, because enrollment reaches the patient by SMS.

The fast loop — no stack needed:

```bash
cd orchestrator
go test ./careplanservice/ -run TestPatientValidator_Validate -v
```

```bash
cd frontend
npx jest __tests__/app/enrollment/new __tests__/lib/fhirUtils.test.ts
```

To see it end-to-end, run the walkthrough above with the patient's phone set to:

| Phone | Expected |
|---|---|
| `+31 6 12345678`, `0612345678` | accepted (Dutch mobile) |
| `+33 6 12 34 56 78`, `0090 532 123 45 67` | accepted (international, `00` → `+`) |
| `020-1234567`, `+31 20 1234567` | `E0004` — Dutch, but not mobile |
| `+33 6 12` | `E0004` — too short for E.164 |

## Rebuilding

- The browser applications hot reload on changes.
- Go applications need a rebuild: `./update.sh` (wraps
  `docker compose up --build -d clinic_orchestrator hospital_orchestrator`).

## Troubleshooting

### `Expected 1 active organization, found 0`, or a blank *Internal Server Error* on app launch

The orchestrator log also shows `Failed to refresh local identities using Nuts node …
subject not found`. The Nuts node keeps its DIDs in the container's writable layer, which
is **not** a volume — so recreating the container discards them, and every app launch
fails until they are re-issued. `docker compose restart nutsnode` is safe; `docker compose
down`, `--force-recreate`, and bringing individual services up (which can recreate the
Nuts node as a dependency) are not.

Re-run the bootstrap:

```bash
docker compose run --rm nutsnode-init
curl -s http://localhost:9081/internal/vdr/v2/subject   # both DIDs back
```

### `hospital_frontend` exits with `Symlink node_modules is invalid, it points out of the filesystem root`

`frontend/` is bind-mounted into the container, so a `node_modules` **symlink** on the host
(handy for running jest in a git worktree) resolves outside the container's root and
Turbopack panics. `hospital_proxy` then dies too, with `host not found in upstream
"hospital_frontend:3000"`. Replace the symlink with a real `node_modules` (or delete it —
the container ships its own) and bring both back up:

```bash
rm frontend/node_modules
docker compose up -d hospital_frontend hospital_proxy
```

### `The "orca_dev_base" variable is not set`

Run `docker compose` from `deployments/dev/`, not from the repository root.
