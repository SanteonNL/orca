# Running a specific version

For instance, to run the test against a locally built image called `ghcr.io/santeonnl/orca_orchestrator:local`, you can use the following command:

```bash
ORCHESTRATOR_IMAGE=ghcr.io/santeonnl/orca_orchestrator:local go test ./...
```