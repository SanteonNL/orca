## Configuration
- `ORCHESTRATOR_NUTS_API_ADDRESS`: address of the Nuts node API to use, e.g. `http://nutsnode:8081`.
- `ORCHESTRATOR_API_LISTEN_ADDRESS`: address to listen on for the API the XIS communicates with, e.g. `:8081`. This address should not be accessible publicly.
- `ORCHESTRATOR_WEB_LISTEN_ADDRESS`: address to listen on for the web interface, e.g. `:8080`, this address should be accessible publicly.
- `ORCHESTRATOR_BASE_URL`: base URL of the Orca deployment, e.g. `http://orca:8080`.
- `ORCHESTRATOR_DEMO_CONFIGFILE`: path to the demo configuration file that contains a map of organization IDs to DIDs. It can also be a string (JSON object) instead of a file.

## TODO
- [ ] Persistence: what to persist and where
- [ ] Test coverage
- [ ] Documentation
- [ ] Employee Identity through integration with Identity Provider