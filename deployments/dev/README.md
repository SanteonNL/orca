# Usage

Run `docker compose up` to start the development environment.

URLs:
- Hospital EHR (task sender): http://localhost:8081/ehr
- Clinic EHR (task receiver): http://localhost:8082/viewer

## Rebuilding

- Browser applications hot reload on changes.
- Golang applications need to be rebuilt and restarted: `docker compose up --build -d clinic_orchestrator hospital_orchestrator` (use `update.sh`)